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
  // Filesystem listing uses parent path for files; for root use null.
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
    <div className="shell-spa">
      <header className="topbar">
        <a className="brand" href="/" onClick={(e) => { e.preventDefault(); onSelect("path:./"); }}>
          refactree
        </a>
        <span className="crumb">
          <code>{isRoot ? data.rootDir : focus}</code>
        </span>
      </header>
      <aside className="rail" aria-label="Navigation">
        <FileRail
          files={files}
          symbols={code?.symbols ?? []}
          activeRef={focus}
          onSelect={onSelect}
        />
      </aside>
      <section className="panel-code">
        {code?.error ? <p className="error">{code.error}</p> : null}
        {code?.warning ? <p className="warning">{code.warning}</p> : null}
        {isRoot && !code?.segments?.length ? (
          <p className="graph-empty lede">
            Local code browser. Pick a file from the rail, or open a reference
            under <code>/code/…</code>. Graph expands from the current focus.
          </p>
        ) : (
          <CodePanel
            segments={code?.segments ?? []}
            nonText={code?.nonText ?? false}
            focusId={code?.focusId ?? null}
            onNavigate={onSelect}
          />
        )}
      </section>
      <section className="panel-graph" aria-label="Relation graph">
        <GraphPanel
          neighborhood={nb}
          onFocus={onSelect}
        />
      </section>
    </div>
  );
}

export function App() {
  return (
    <React.Suspense fallback={<p className="muted" style={{ padding: "1rem" }}>Loading…</p>}>
      <Shell />
    </React.Suspense>
  );
}
