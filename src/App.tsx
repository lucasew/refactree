import React, { useCallback, useEffect, useState } from "react";
import { graphql, useLazyLoadQuery } from "react-relay";
import type { AppRootQuery } from "./__generated__/AppRootQuery.graphql";
import { FileRail } from "./components/FileRail";
import { CodePanel } from "./components/CodePanel";
import { GraphPanel } from "./components/GraphPanel";
import { navigateToRef, refFromPath } from "./routes";

const RootQuery = graphql`
  query AppRootQuery($fsRef: ID, $focus: ID!, $hasFocus: Boolean!) {
    rootDir
    filesystem(ref: $fsRef) {
      name
      reference
      isDir
    }
    neighborhood(ref: $focus) @include(if: $hasFocus) {
      incomplete
      focus {
        id
        kind
        label
        parentId
      }
      nodes {
        id
        kind
        label
        parentId
      }
      edges {
        from
        to
        kind
      }
    }
    code(ref: $focus) @include(if: $hasFocus) {
      reference
      language
      nonText
      error
      warning
      focusId
      parentHref
      segments {
        text
        href
        anchorId
        isLink
        isDef
        reference
      }
      files {
        name
        reference
        isDir
      }
      symbols {
        name
        reference
        isDir
      }
    }
  }
`;

function Shell() {
  const [path, setPath] = useState(() => window.location.pathname);

  useEffect(() => {
    const onPop = () => setPath(window.location.pathname);
    window.addEventListener("popstate", onPop);
    return () => window.removeEventListener("popstate", onPop);
  }, []);

  const focus = refFromPath(path);
  const isRoot = focus === "path:./" || focus === "path:.";
  const fsRef = isRoot ? null : focus;

  const data = useLazyLoadQuery<AppRootQuery>(
    RootQuery,
    {
      fsRef: isRoot ? null : fsRef,
      focus: isRoot ? "path:./" : focus,
      hasFocus: true,
    },
    { fetchPolicy: "store-and-network" }
  );

  const onSelect = useCallback((ref: string) => {
    navigateToRef(ref);
  }, []);

  const nb = data.neighborhood;
  const code = data.code;
  const files = code?.files?.length ? code.files : data.filesystem ?? [];

  return (
    <div className="flex min-h-dvh flex-col">
      <div className="navbar bg-base-200 border-b border-base-300 min-h-12 px-3">
        <div className="navbar-start gap-2">
          <a
            href="/"
            className="btn btn-ghost btn-sm text-primary font-bold normal-case"
            onClick={(e) => {
              e.preventDefault();
              onSelect("path:./");
            }}
          >
            refactree
          </a>
        </div>
        <div className="navbar-center max-w-[70vw] truncate">
          <code className="text-xs text-base-content/70 font-mono">
            {isRoot ? data.rootDir : focus}
          </code>
        </div>
        <div className="navbar-end" />
      </div>

      <div className="grid flex-1 grid-cols-1 lg:grid-cols-[16rem_1fr_1fr] min-h-0">
        <aside
          className="border-b lg:border-b-0 lg:border-r border-base-300 bg-base-200 overflow-auto max-h-[40vh] lg:max-h-[calc(100dvh-3rem)]"
          aria-label="Navigation"
        >
          <FileRail
            files={files}
            symbols={code?.symbols ?? []}
            activeRef={focus}
            onSelect={onSelect}
          />
        </aside>

        <section className="overflow-auto max-h-[50vh] lg:max-h-[calc(100dvh-3rem)] border-b lg:border-b-0 lg:border-r border-base-300 bg-base-100">
          {code?.error ? (
            <div role="alert" className="alert alert-error alert-soft m-2 text-sm">
              <span>{code.error}</span>
            </div>
          ) : null}
          {code?.warning ? (
            <div role="alert" className="alert alert-warning alert-soft m-2 text-sm">
              <span>{code.warning}</span>
            </div>
          ) : null}
          {isRoot && !code?.segments?.length ? (
            <div className="p-4 text-sm text-base-content/70">
              <p>
                Local code browser. Pick a file from the rail, or open a reference under{" "}
                <code className="kbd kbd-sm">/code/…</code>. Graph expands from the current focus.
              </p>
            </div>
          ) : (
            <CodePanel
              segments={code?.segments ?? []}
              nonText={code?.nonText ?? false}
              focusId={code?.focusId ?? null}
              onNavigate={onSelect}
            />
          )}
        </section>

        <section
          className="relative min-h-64 lg:min-h-0 max-h-[50vh] lg:max-h-[calc(100dvh-3rem)] bg-base-100"
          aria-label="Relation graph"
        >
          <GraphPanel neighborhood={nb} onFocus={onSelect} />
        </section>
      </div>
    </div>
  );
}

export function App() {
  return (
    <React.Suspense
      fallback={
        <div className="flex min-h-dvh items-center justify-center gap-2 text-base-content/70">
          <span className="loading loading-spinner loading-sm" />
          Loading…
        </div>
      }
    >
      <Shell />
    </React.Suspense>
  );
}
