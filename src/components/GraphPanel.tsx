import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import ForceGraph2D from "react-force-graph-2d";
import {
  getGraphSession,
  isExternalId,
  mergeNeighborhood,
  snapshotGraphData,
  viewFocusId,
  type IncomingNeighborhood,
} from "../graphSession";
import {
  ensureGraphSession,
  sessionProject,
  sessionVisit,
  streamExpandExternal,
} from "../graphStream";
import { nodeFill, nodeStroke, inferLanguageFromId } from "../graphColors";
import {
  formatGraphLabel,
  normalizeRef,
  type GraphViewMode,
} from "../routes";
import {
  computeUsage,
  forceUsageGravity,
  forceUsageRadial,
  degreeRadiusBoost,
} from "../graphForces";

type Props = {
  focusId: string;
  neighborhood?: IncomingNeighborhood;
  /** Visit this ref in the shared graph session (deltas). */
  streamRef?: string | null;
  /** Load project import map into the same session. */
  streamProject?: boolean;
  loading?: boolean;
  onFocus: (ref: string) => void;
  onExpandExternal?: (ref: string) => void;
  emptyHint?: string;
};

const BG = "#1a1814";

export function GraphPanel({
  focusId,
  neighborhood,
  streamRef,
  streamProject,
  loading: loadingProp,
  onFocus,
  onExpandExternal,
  emptyHint,
}: Props) {
  const hostRef = useRef<HTMLDivElement>(null);
  const fgRef = useRef<any>(null);
  const [size, setSize] = useState({ w: 0, h: 0 });
  const [tick, setTick] = useState(0);
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState<string | null>(null);
  /**
   * package = collapse to one node per package (path:./cmd/rft once)
   * reference = full refs (path:./cmd/rft::Main distinct)
   */
  const [viewMode, setViewMode] = useState<GraphViewMode>("reference");
  const lastVisit = useRef<string>("");

  const bump = useCallback(() => {
    setTick((t) => t + 1);
    try {
      fgRef.current?.d3ReheatSimulation?.();
    } catch {
      /* ignore */
    }
  }, []);

  useEffect(() => {
    ensureGraphSession();
  }, []);

  useEffect(() => {
    const el = hostRef.current;
    if (!el) return;
    const measure = () => {
      const r = el.getBoundingClientRect();
      const w = Math.max(0, Math.floor(r.width));
      const h = Math.max(0, Math.floor(r.height));
      setSize((prev) => (prev.w === w && prev.h === h ? prev : { w, h }));
    };
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  // Project map once when requested (session accumulates).
  useEffect(() => {
    if (!streamProject) return;
    let cancelled = false;
    setBusy(true);
    setErr(null);
    sessionProject({
      onEvent: () => {
        if (!cancelled) bump();
      },
      onError: (e) => {
        if (!cancelled) setErr(e.message);
      },
    }).finally(() => {
      if (!cancelled) setBusy(false);
    });
    return () => {
      cancelled = true;
    };
  }, [streamProject, bump]);

  // Visit focus: same WS session, only new edges arrive.
  useEffect(() => {
    if (streamProject) return;
    if (streamRef == null || streamRef === "") return;
    const visitRef = normalizeRef(streamRef);
    if (lastVisit.current === visitRef && getGraphSession().nodes.size > 0) {
      // still re-visit so server can push any new edges; always visit is ok (deltas)
    }
    lastVisit.current = visitRef;
    let cancelled = false;
    setBusy(true);
    setErr(null);
    sessionVisit(visitRef, {
      onEvent: () => {
        if (!cancelled) bump();
      },
      onError: (e) => {
        if (!cancelled) setErr(e.message);
      },
    }).finally(() => {
      if (!cancelled) setBusy(false);
    });
    return () => {
      cancelled = true;
    };
  }, [streamRef, streamProject, bump]);

  useEffect(() => {
    if (!neighborhood || streamRef != null || streamProject) return;
    mergeNeighborhood(neighborhood, focusId);
    bump();
  }, [neighborhood, focusId, streamRef, streamProject, bump]);

  useEffect(() => {
    getGraphSession().focusId = normalizeRef(focusId);
    bump();
  }, [focusId, bump]);

  const graphData = useMemo(() => {
    void tick;
    return snapshotGraphData(viewMode);
  }, [tick, viewMode]);

  const focusInView = useMemo(
    () => viewFocusId(getGraphSession().focusId || focusId, viewMode),
    [tick, focusId, viewMode]
  );

  // Indegree usage: imported/used-by-many → center; unused → rim.
  const usage = useMemo(() => computeUsage(graphData.links), [graphData.links]);

  useEffect(() => {
    const fg = fgRef.current;
    if (!fg || graphData.nodes.length === 0) return;

    const getUse = (id: string) => usage.get(id) ?? 0;

    // Soften default center so usage forces dominate placement.
    try {
      const center = fg.d3Force("center");
      if (center && typeof center.strength === "function") {
        center.strength(0.015);
      }
    } catch {
      /* ignore */
    }

    // Unused nodes repel harder (spread on rim); hubs cluster.
    try {
      const charge = fg.d3Force("charge");
      if (charge && typeof charge.strength === "function") {
        charge.strength((node: { id?: string }) => {
          const u = getUse(node.id ?? "");
          return -24 - Math.max(0, 10 - u) * 5;
        });
      }
    } catch {
      /* ignore */
    }

    // Links between highly used nodes slightly shorter.
    try {
      const link = fg.d3Force("link");
      if (link && typeof link.distance === "function") {
        link.distance((l: { source?: { id?: string } | string; target?: { id?: string } | string }) => {
          const idOf = (x: unknown) =>
            typeof x === "string" ? x : (x as { id?: string })?.id ?? "";
          const a = getUse(idOf(l.source));
          const b = getUse(idOf(l.target));
          const avg = (a + b) / 2;
          return 42 + Math.max(0, 14 - avg) * 3;
        });
      }
    } catch {
      /* ignore */
    }

    fg.d3Force("usageGravity", forceUsageGravity(getUse));
    fg.d3Force(
      "usageRadial",
      forceUsageRadial(getUse, {
        outer: Math.min(size.w, size.h) * 0.4 || 200,
        inner: 14,
        strength: 0.14,
      })
    );

    try {
      fg.d3ReheatSimulation?.();
    } catch {
      /* ignore */
    }
  }, [usage, graphData.nodes.length, size.w, size.h, tick]);

  const onNodeClick = useCallback(
    (node: { id: string; expandable?: boolean; external?: boolean }) => {
      const external = node.external || isExternalId(node.id);
      if (external && node.expandable !== false) {
        if (onExpandExternal) {
          onExpandExternal(node.id);
          return;
        }
        setBusy(true);
        streamExpandExternal(normalizeRef(node.id), { onEvent: bump })
          .catch((e) => setErr(String(e)))
          .finally(() => setBusy(false));
        return;
      }
      onFocus(normalizeRef(node.id));
    },
    [onFocus, onExpandExternal, bump]
  );

  const loading = loadingProp || busy;
  const sess = getGraphSession();

  if (graphData.nodes.length === 0) {
    return (
      <div ref={hostRef} className="graph-canvas-host relative h-full min-h-64">
        <div className="p-4 text-sm text-base-content/60 flex items-center gap-2">
          {loading ? <span className="loading loading-spinner loading-xs" /> : null}
          {err ? (
            <span className="text-error">{err}</span>
          ) : loading ? (
            "Session: discovering edges…"
          ) : (
            emptyHint ?? "Visit files and symbols — the graph session accumulates edges."
          )}
        </div>
      </div>
    );
  }

  return (
    <div ref={hostRef} className="graph-canvas-host relative h-full min-h-64 overflow-hidden">
      <div className="absolute z-10 m-2 flex flex-wrap items-center gap-1">
        <span className="badge badge-ghost badge-sm">
          {graphData.nodes.length}n · {graphData.links.length}e
        </span>
        {sess.incomplete ? (
          <span className="badge badge-ghost badge-sm">incomplete</span>
        ) : null}
        {loading ? (
          <span className="badge badge-primary badge-outline badge-sm gap-1">
            <span className="loading loading-spinner loading-xs" />
            session
          </span>
        ) : (
          <span className="badge badge-ghost badge-sm opacity-70">session live</span>
        )}
        {err ? <span className="badge badge-error badge-sm">{err}</span> : null}
        <div className="join" role="group" aria-label="Graph view mode">
          <button
            type="button"
            className={`btn btn-xs join-item ${viewMode === "package" ? "btn-active" : ""}`}
            title="One node per package (path:./cmd/rft once)"
            onClick={() => setViewMode("package")}
          >
            package
          </button>
          <button
            type="button"
            className={`btn btn-xs join-item ${viewMode === "reference" ? "btn-active" : ""}`}
            title="One node per full ref (package/name)"
            onClick={() => setViewMode("reference")}
          >
            reference
          </button>
        </div>
        <span
          className="badge badge-ghost badge-sm opacity-60"
          title="Used-by many → center; unused → rim. Fill = language; ring = path vs external."
        >
          gravity: used→center
        </span>
      </div>
      {size.w > 0 && size.h > 0 ? (
        <ForceGraph2D
          ref={fgRef}
          width={size.w}
          height={size.h}
          graphData={graphData}
          nodeId="id"
          nodeLabel={(n: any) => {
            const lang = n.language || inferLanguageFromId(n.id) || "?";
            const scope = n.external ? "external" : "path";
            const tip = n.external ? " — click to expand" : "";
            const pretty = formatGraphLabel(n.id, viewMode).replace(/\n/g, " · ");
            const u = usage.get(n.id) ?? 0;
            return `${pretty} [${lang} · ${scope} · used by ${u}]${tip}`;
          }}
          backgroundColor={BG}
          linkWidth={1.25}
          linkColor={(l: any) => {
            const kind = l.kind as string;
            if (kind === "IMPORTS") return "#7a8aaa";
            if (kind === "USED_BY") return "#aa8a7a";
            return "#8aaa7a";
          }}
          linkDirectionalArrowLength={4}
          linkDirectionalArrowRelPos={1}
          linkCurvature={0.05}
          nodeCanvasObjectMode={() => "replace"}
          nodeCanvasObject={(node: any, ctx, globalScale) => {
            const label = formatGraphLabel(node.id, viewMode);
            const fontSize = Math.max(10 / globalScale, 2);
            const baseR =
              viewMode === "package"
                ? 7
                : node.kind === "ATOM"
                  ? 4
                  : node.kind === "MODULE"
                    ? 7
                    : 5;
            const r = degreeRadiusBoost(usage.get(node.id) ?? 0, baseR);
            const x = node.x ?? 0;
            const y = node.y ?? 0;
            const lang = node.language || inferLanguageFromId(node.id);
            const focused = node.id === focusInView;
            const external = !!node.external;
            ctx.beginPath();
            ctx.arc(x, y, r, 0, 2 * Math.PI, false);
            ctx.fillStyle = nodeFill({
              language: lang,
              external,
              focused,
            });
            ctx.fill();
            const stroke = nodeStroke({
              external,
              focused,
              expandable: node.expandable,
            });
            if (stroke) {
              ctx.strokeStyle = stroke.color;
              ctx.lineWidth = stroke.width / globalScale;
              if (external && !focused) {
                ctx.setLineDash([3 / globalScale, 2 / globalScale]);
              } else {
                ctx.setLineDash([]);
              }
              ctx.stroke();
              ctx.setLineDash([]);
            }
            if (globalScale > 0.55) {
              const lines = String(label).split("\n").filter(Boolean);
              ctx.textAlign = "center";
              ctx.textBaseline = "top";
              let yy = y + r + 2;
              lines.forEach((line: string, i: number) => {
                // package line muted when reference shows two lines
                if (lines.length > 1 && i === 0) {
                  ctx.font = `${Math.max(fontSize * 0.85, 1.5)}px sans-serif`;
                  ctx.fillStyle = "rgba(232,224,208,0.7)";
                } else {
                  ctx.font = `${fontSize}px sans-serif`;
                  ctx.fillStyle = "#e8e0d0";
                }
                ctx.fillText(line, x, yy);
                yy += fontSize * 1.15;
              });
            }
          }}
          nodePointerAreaPaint={(node: any, color, ctx) => {
            ctx.beginPath();
            ctx.arc(node.x ?? 0, node.y ?? 0, 10, 0, 2 * Math.PI, false);
            ctx.fillStyle = color;
            ctx.fill();
          }}
          onNodeClick={onNodeClick}
          cooldownTicks={100}
          autoPauseRedraw={false}
          enableNodeDrag={true}
        />
      ) : null}
    </div>
  );
}
