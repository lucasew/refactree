import React, { useCallback } from "react";
import { graphql } from "react-relay";
import type { ProjectGraphPageRootQuery as RootQ } from "./__generated__/ProjectGraphPageRootQuery.graphql";
import { GraphPanel } from "./GraphPanel";
import { useRelayQuery } from "../useRelayQuery";
import { navigateToRef } from "../routes";

const RootQuery = graphql`
  query ProjectGraphPageRootQuery {
    rootDir
  }
`;

type Props = {
  onPathChange: () => void;
};

export function ProjectGraphPage({ onPathChange }: Props) {
  const root = useRelayQuery<RootQ>(RootQuery, {}, "projectRoot");

  const onFocus = useCallback(
    (ref: string) => {
      // path: and external (go:, node:, …) — open definition in the code browser
      navigateToRef(ref);
      onPathChange();
    },
    [onPathChange]
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
            {root.data?.rootDir ?? "…"}
          </code>
        </div>
        <div className="navbar-end gap-2">
          <a
            href="/"
            className="btn btn-ghost btn-xs"
            onClick={(e) => {
              e.preventDefault();
              navigateToRef("path:./");
              onPathChange();
            }}
          >
            code browser
          </a>
        </div>
      </div>

      <div className="flex-1 min-h-0 relative" style={{ height: "calc(100dvh - 3rem)" }}>
        <GraphPanel
          focusId="path:./"
          streamProject
          onFocus={onFocus}
          emptyHint="Turn on “crawl repo” (or wait) to stream the project import graph…"
        />
        <p className="absolute bottom-2 left-2 text-xs text-base-content/50 max-w-lg">
          Crawl repo walks the tree with the usual skip list (node_modules, .venv, vendor, …). Orange-ring nodes are
          external — click to open source and expand. Path nodes open the code browser.
        </p>
      </div>
    </div>
  );
}
