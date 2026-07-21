import React, { useCallback, useEffect, useMemo, useState } from "react";
import { graphql } from "react-relay";
import type { AppFilesystemQuery as FsQ } from "./__generated__/AppFilesystemQuery.graphql";
import type { AppCodeQuery as CodeQ } from "./__generated__/AppCodeQuery.graphql";
import type { AppRootDirQuery as RootQ } from "./__generated__/AppRootDirQuery.graphql";
import { FileRail } from "./components/FileRail";
import { CodePanel } from "./components/CodePanel";
import { GraphPanel } from "./components/GraphPanel";
import { ProjectGraphPage } from "./components/ProjectGraphPage";
import { isGraphRoute, navigateToGraph, navigateToRef, refFromPath } from "./routes";
import { mergeCodeLinksIntoGraph } from "./graphStream";
import { useRelayQuery } from "./useRelayQuery";

const RootDirQuery = graphql`
  query AppRootDirQuery {
    rootDir
  }
`;

const FilesystemQuery = graphql`
  query AppFilesystemQuery($fsRef: ID) {
    filesystem(ref: $fsRef) {
      name
      reference
      isDir
    }
  }
`;

const CodeQuery = graphql`
  query AppCodeQuery($focus: ID!) {
    code(ref: $focus) {
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

function PanelPending({ label }: { label: string }) {
  return (
    <div className="flex items-center gap-2 p-3 text-sm text-base-content/50">
      <span className="loading loading-spinner loading-xs" />
      {label}
    </div>
  );
}

function CodeBrowser({ path, onPathChange }: { path: string; onPathChange: () => void }) {
  const focus = refFromPath(path);
  const isRoot = focus === "path:./" || focus === "path:.";
  const codeFocus = isRoot ? "path:./" : focus;
  const fsRef = isRoot ? null : focus;

  const root = useRelayQuery<RootQ>(RootDirQuery, {}, "root");
  const fs = useRelayQuery<FsQ>(
    FilesystemQuery,
    { fsRef },
    `fs:${fsRef ?? ""}`
  );
  const code = useRelayQuery<CodeQ>(
    CodeQuery,
    { focus: codeFocus },
    `code:${codeFocus}`
  );

  const onSelect = useCallback(
    (ref: string) => {
      navigateToRef(ref);
      onPathChange();
    },
    [onPathChange]
  );

  const codeData = code.data?.code;
  const files = useMemo(() => {
    if (codeData?.files?.length) return codeData.files;
    return fs.data?.filesystem ?? [];
  }, [codeData?.files, fs.data?.filesystem]);

  const symbols = codeData?.symbols ?? [];

  // File browser → graph: paint the same hyperlinks the code pane just resolved
  // without waiting for the WS visit worker (shared server corpus still primes cache).
  useEffect(() => {
    if (!codeData || isRoot) return;
    const links: Array<{ reference?: string | null; isLink?: boolean | null }> = [];
    for (const s of codeData.segments ?? []) {
      if (s?.isLink && s.reference) links.push({ reference: s.reference, isLink: true });
    }
    for (const s of codeData.symbols ?? []) {
      if (s?.reference) links.push({ reference: s.reference, isLink: true });
    }
    if (links.length) mergeCodeLinksIntoGraph(codeFocus, links);
  }, [codeData, codeFocus, isRoot]);

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
        <div className="navbar-center max-w-[50vw] truncate">
          <code className="text-xs text-base-content/70 font-mono">
            {isRoot ? root.data?.rootDir ?? "…" : focus}
          </code>
        </div>
        <div className="navbar-end gap-2">
          {(fs.loading || code.loading) && (
            <span className="loading loading-spinner loading-xs opacity-60" title="Loading…" />
          )}
          <a
            href="/graph"
            className="btn btn-ghost btn-xs"
            onClick={(e) => {
              e.preventDefault();
              navigateToGraph();
              onPathChange();
            }}
          >
            project graph
          </a>
        </div>
      </div>

      <div className="grid flex-1 grid-cols-1 lg:grid-cols-[16rem_1fr_1fr] min-h-0">
        <aside
          className="border-b lg:border-b-0 lg:border-r border-base-300 bg-base-200 overflow-auto max-h-[40vh] lg:max-h-[calc(100dvh-3rem)]"
          aria-label="Navigation"
        >
          {fs.error ? (
            <div role="alert" className="alert alert-error alert-soft m-2 text-xs">
              {fs.error.message}
            </div>
          ) : null}
          {fs.loading && !fs.data ? (
            <PanelPending label="Listing…" />
          ) : (
            <FileRail
              files={files}
              symbols={symbols}
              activeRef={focus}
              onSelect={onSelect}
              symbolsLoading={code.loading && !codeData}
            />
          )}
        </aside>

        <section className="overflow-auto max-h-[50vh] lg:max-h-[calc(100dvh-3rem)] border-b lg:border-b-0 lg:border-r border-base-300 bg-base-100">
          {code.error ? (
            <div role="alert" className="alert alert-error alert-soft m-2 text-sm">
              <span>{code.error.message}</span>
            </div>
          ) : null}
          {codeData?.error ? (
            <div role="alert" className="alert alert-error alert-soft m-2 text-sm">
              <span>{codeData.error}</span>
            </div>
          ) : null}
          {codeData?.warning ? (
            <div role="alert" className="alert alert-warning alert-soft m-2 text-sm">
              <span>{codeData.warning}</span>
            </div>
          ) : null}
          {code.loading && !codeData ? (
            <PanelPending label="Loading source…" />
          ) : isRoot && !codeData?.segments?.length ? (
            <div className="p-4 text-sm text-base-content/70">
              <p>
                Local code browser. Pick a file, open{" "}
                <a
                  href="/graph"
                  className="link"
                  onClick={(e) => {
                    e.preventDefault();
                    navigateToGraph();
                    onPathChange();
                  }}
                >
                  project graph
                </a>
                , or a <code className="kbd kbd-sm">/code/…</code> reference. Graph streams in
                as relations are discovered.
              </p>
            </div>
          ) : (
            <CodePanel
              segments={codeData?.segments ?? []}
              nonText={codeData?.nonText ?? false}
              focusId={codeData?.focusId ?? null}
              onNavigate={onSelect}
              loading={code.loading}
            />
          )}
        </section>

        <section
          className="relative min-h-64 lg:min-h-0 max-h-[50vh] lg:max-h-[calc(100dvh-3rem)] bg-base-100"
          aria-label="Relation graph"
        >
          <GraphPanel
            focusId={codeFocus}
            streamRef={codeFocus}
            onFocus={onSelect}
          />
        </section>
      </div>
    </div>
  );
}

export function App() {
  // pathname + hash so symbol deep-links re-scroll when only the fragment changes
  const [path, setPath] = useState(
    () => window.location.pathname + window.location.hash
  );
  const bump = useCallback(
    () => setPath(window.location.pathname + window.location.hash),
    []
  );

  useEffect(() => {
    const onPop = () => setPath(window.location.pathname + window.location.hash);
    window.addEventListener("popstate", onPop);
    window.addEventListener("hashchange", onPop);
    return () => {
      window.removeEventListener("popstate", onPop);
      window.removeEventListener("hashchange", onPop);
    };
  }, []);

  const pathname = path.split("#")[0] || "/";
  if (isGraphRoute(pathname)) {
    return <ProjectGraphPage onPathChange={bump} />;
  }
  return <CodeBrowser path={pathname} onPathChange={bump} />;
}
