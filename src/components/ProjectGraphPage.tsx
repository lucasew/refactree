import React, { useCallback, useEffect, useState } from "react";
import { graphql } from "react-relay";
import type { ProjectGraphPageQuery as PGQ } from "../__generated__/ProjectGraphPageQuery.graphql";
import type { ProjectGraphPageExpandQuery as EXQ } from "../__generated__/ProjectGraphPageExpandQuery.graphql";
import { GraphPanel } from "./GraphPanel";
import { useRelayQuery } from "../useRelayQuery";
import { environment } from "../RelayEnvironment";
import { fetchQuery } from "relay-runtime";
import { navigateToRef } from "../routes";
import { markExpanded, mergeNeighborhood } from "../graphSession";

const ProjectGraphQuery = graphql`
  query ProjectGraphPageQuery {
    rootDir
    projectGraph {
      incomplete
      focus {
        id
        kind
        label
        parentId
        external
        expandable
      }
      nodes {
        id
        kind
        label
        parentId
        external
        expandable
      }
      edges {
        from
        to
        kind
      }
    }
  }
`;

const ExpandQuery = graphql`
  query ProjectGraphPageExpandQuery($focus: ID!) {
    neighborhood(ref: $focus) {
      incomplete
      focus {
        id
        kind
        label
        parentId
        external
        expandable
      }
      nodes {
        id
        kind
        label
        parentId
        external
        expandable
      }
      edges {
        from
        to
        kind
      }
    }
  }
`;

type Props = {
  onPathChange: () => void;
};

export function ProjectGraphPage({ onPathChange }: Props) {
  const pg = useRelayQuery<PGQ>(ProjectGraphQuery, {}, "projectGraph");
  const [expanding, setExpanding] = useState<string | null>(null);

  // Force GraphPanel to re-read session after expands.
  const [localNb, setLocalNb] = useState(pg.data?.projectGraph ?? null);
  useEffect(() => {
    if (pg.data?.projectGraph) {
      setLocalNb(pg.data.projectGraph);
    }
  }, [pg.data?.projectGraph]);

  const onExpandExternal = useCallback(async (ref: string) => {
    setExpanding(ref);
    try {
      const data = await fetchQuery<EXQ>(environment, ExpandQuery, { focus: ref }).toPromise();
      if (data?.neighborhood) {
        mergeNeighborhood(data.neighborhood, ref);
        markExpanded(ref);
        // nudge panel with a shallow copy of last project graph + flag
        setLocalNb((prev) =>
          prev
            ? {
                ...prev,
                incomplete: true,
                nodes: [...(prev.nodes ?? [])],
                edges: [...(prev.edges ?? [])],
              }
            : prev
        );
      }
    } finally {
      setExpanding(null);
    }
  }, []);

  const onFocus = useCallback(
    (ref: string) => {
      // path: open code view; external expand handled separately
      if (ref.startsWith("path:") || ref.includes("://")) {
        navigateToRef(ref);
        onPathChange();
        return;
      }
      // non-path: expand by default
      void onExpandExternal(ref);
    },
    [onExpandExternal, onPathChange]
  );

  return (
    <div className="flex min-h-dvh flex-col">
      <div className="navbar bg-base-200 border-b border-base-300 min-h-12 px-3">
        <div className="navbar-start gap-2">
          <a
            href="/"
            className="btn btn-ghost btn-sm text-primary font-bold normal-case"
            onClick={(e) => {
              e.preventDefault();
              navigateToRef("path:./");
              onPathChange();
            }}
          >
            refactree
          </a>
          <span className="badge badge-primary badge-outline badge-sm">project graph</span>
        </div>
        <div className="navbar-center max-w-[60vw] truncate">
          <code className="text-xs text-base-content/70 font-mono">
            {pg.data?.rootDir ?? "…"}
          </code>
        </div>
        <div className="navbar-end gap-2">
          {(pg.loading || expanding) && (
            <span className="loading loading-spinner loading-xs opacity-60" />
          )}
          <a href="/" className="btn btn-ghost btn-xs" onClick={(e) => {
            e.preventDefault();
            navigateToRef("path:./");
            onPathChange();
          }}>
            code browser
          </a>
        </div>
      </div>

      <div className="flex-1 min-h-0 relative" style={{ height: "calc(100dvh - 3rem)" }}>
        {pg.error ? (
          <div role="alert" className="alert alert-error m-4">
            {pg.error.message}
          </div>
        ) : null}
        <GraphPanel
          focusId="path:./"
          neighborhood={localNb as any}
          loading={pg.loading || !!expanding}
          onFocus={onFocus}
          onExpandExternal={onExpandExternal}
          emptyHint="Loading project import graph…"
        />
        <p className="absolute bottom-2 left-2 text-xs text-base-content/50 max-w-md">
          Orange-ring nodes are external (non-path) deps. Click to lazy-expand their
          neighborhood. Path nodes open the code browser. Positions persist in this session.
        </p>
      </div>
    </div>
  );
}
