import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import ForceGraph2D from "react-force-graph-2d";
import {
  getGraphSession,
  isExternalId,
  mergeNeighborhood,
  snapshotGraphData,
  type IncomingNeighborhood,
} from "../graphSession";
import {
  ensureGraphSession,
  sessionProject,
  sessionVisit,
  streamExpandExternal,
} from "../graphStream";
import { nodeFill, nodeStroke, inferLanguageFromId } from "../graphColors";

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
    if (lastVisit.current === streamRef && getGraphSession().nodes.size > 0) {
      // still re-visit so server can push any new edges; always visit is ok (deltas)
    }
    lastVisit.current = streamRef;
    let cancelled = false;
    setBusy(true);
    setErr(null);
    sessionVisit(streamRef, {
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
    getGraphSession().focusId = focusId;
    bump();
  }, [focusId, bump]);

  const graphData = useMemo(() => {
    void tick;
    return snapshotGraphData();
  }, [tick]);

  const onNodeClick = useCallback(
    (node: { id: string; expandable?: boolean; external?: boolean }) => {
      const external = node.external || isExternalId(node.id);
      if (external && node.expandable !== false) {
        if (onExpandExternal) {
          onExpandExternal(node.id);
          return;
        }
        setBusy(true);
        streamExpandExternal(node.id, { onEvent: bump })
          .catch((e) => setErr(String(e)))
          .finally(() => setBusy(false));
        return;
      }
      onFocus(node.id);
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
      <div className="absolute z-10 m-2 flex flex-wrap gap-1">
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
        <span className="badge badge-ghost badge-sm opacity-60" title="Fill = language; solid ring = path (internal); dashed amber = external provider">
          fill: lang · ring: path/external
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
            return `${n.name || n.id} [${lang} · ${scope}]${tip}`;
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
            const label = node.name || node.id;
            const fontSize = Math.max(10 / globalScale, 2);
            const r = node.kind === "ATOM" ? 4 : node.kind === "MODULE" ? 7 : 5;
            const x = node.x ?? 0;
            const y = node.y ?? 0;
            const lang = node.language || inferLanguageFromId(node.id);
            const focused = node.id === sess.focusId;
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
              // Dashed ring for external, solid for focus/internal
              if (external && !focused) {
                ctx.setLineDash([3 / globalScale, 2 / globalScale]);
              } else {
                ctx.setLineDash([]);
              }
              ctx.stroke();
              ctx.setLineDash([]);
            }
            if (globalScale > 0.6) {
              ctx.font = `${fontSize}px sans-serif`;
              ctx.textAlign = "center";
              ctx.textBaseline = "top";
              ctx.fillStyle = "#e8e0d0";
              ctx.fillText(label, x, y + r + 1);
            }
          }}
          nodePointerAreaPaint={(node: any, color, ctx) => {
            ctx.beginPath();
            ctx.arc(node.x ?? 0, node.y ?? 0, 10, 0, 2 * Math.PI, false);
            ctx.fillStyle = color;
            ctx.fill();
          }}
          onNodeClick={onNodeClick}
          cooldownTicks={80}
          autoPauseRedraw={false}
          enableNodeDrag={true}
        />
      ) : null}
    </div>
  );
}
