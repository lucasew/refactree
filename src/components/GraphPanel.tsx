import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import ForceGraph2D from "react-force-graph-2d";
import {
  getGraphSession,
  isExternalId,
  mergeNeighborhood,
  snapshotGraphData,
  type IncomingNeighborhood,
} from "../graphSession";
import { streamExpandExternal, streamGraph } from "../graphStream";

type Props = {
  focusId: string;
  /** When set, merges bulk neighborhood (fallback). Prefer streamKey for progressive load. */
  neighborhood?: IncomingNeighborhood;
  /** When this changes, start SSE stream for focus (neighborhood mode). */
  streamRef?: string | null;
  /** Stream project graph instead of a focus ref. */
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
  const [streaming, setStreaming] = useState(false);
  const [streamError, setStreamError] = useState<string | null>(null);

  const bump = useCallback(() => {
    setTick((t) => t + 1);
    const fg = fgRef.current;
    try {
      // light reheat as graph grows
      fg?.d3ReheatSimulation?.();
    } catch {
      /* ignore */
    }
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

  // Progressive SSE stream
  useEffect(() => {
    if (!streamProject && (streamRef == null || streamRef === "")) return;
    const ac = new AbortController();
    setStreaming(true);
    setStreamError(null);
    const run = streamProject
      ? streamGraph({ mode: "project" }, { onEvent: bump }, ac.signal)
      : streamGraph({ ref: streamRef!, mode: "neighborhood" }, { onEvent: bump }, ac.signal);
    run
      .then(() => {
        if (!ac.signal.aborted) setStreaming(false);
      })
      .catch((e: Error) => {
        if (ac.signal.aborted) return;
        setStreaming(false);
        setStreamError(e.message);
      });
    return () => ac.abort();
  }, [streamRef, streamProject, bump]);

  // Optional bulk merge (compat)
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
        // Default: stream expand in place
        const ac = new AbortController();
        setStreaming(true);
        streamExpandExternal(node.id, { onEvent: bump }, ac.signal)
          .then(() => setStreaming(false))
          .catch(() => setStreaming(false));
        return;
      }
      onFocus(node.id);
    },
    [onFocus, onExpandExternal, bump]
  );

  const loading = loadingProp || streaming;
  const sess = getGraphSession();

  if (graphData.nodes.length === 0) {
    return (
      <div ref={hostRef} className="graph-canvas-host relative h-full min-h-64">
        <div className="p-4 text-sm text-base-content/60 flex items-center gap-2">
          {loading ? <span className="loading loading-spinner loading-xs" /> : null}
          {streamError ? (
            <span className="text-error">{streamError}</span>
          ) : loading ? (
            "Streaming graph…"
          ) : (
            emptyHint ?? "Focus a file or symbol to expand the relation graph."
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
            streaming
          </span>
        ) : null}
        {streamError ? (
          <span className="badge badge-error badge-sm">{streamError}</span>
        ) : null}
      </div>
      {size.w > 0 && size.h > 0 ? (
        <ForceGraph2D
          ref={fgRef}
          width={size.w}
          height={size.h}
          graphData={graphData}
          nodeId="id"
          nodeLabel={(n: any) =>
            n.external
              ? `${n.name || n.id} (external — click to expand)`
              : n.name || n.id
          }
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
            ctx.beginPath();
            ctx.arc(x, y, r, 0, 2 * Math.PI, false);
            if (node.external) {
              ctx.fillStyle = node.id === sess.focusId ? "#e8a838" : "#c45c26";
            } else {
              ctx.fillStyle =
                node.id === sess.focusId
                  ? "#e8a838"
                  : node.kind === "ATOM"
                    ? "#6bcf8e"
                    : node.kind === "MODULE"
                      ? "#7aa2f7"
                      : "#a0a0a0";
            }
            ctx.fill();
            if (node.external && node.expandable !== false) {
              ctx.strokeStyle = "#e8a838";
              ctx.lineWidth = 1.5 / globalScale;
              ctx.stroke();
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
