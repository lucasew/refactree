import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import ForceGraph2D from "react-force-graph-2d";
import {
  getGraphSession,
  isExternalId,
  markExpanded,
  mergeNeighborhood,
  snapshotGraphData,
  type IncomingNeighborhood,
} from "../graphSession";

type Props = {
  focusId: string;
  neighborhood: IncomingNeighborhood;
  loading?: boolean;
  onFocus: (ref: string) => void;
  /** Called when user requests expand of an external (non-path) node. */
  onExpandExternal?: (ref: string) => void;
  emptyHint?: string;
};

const BG = "#1a1814";

export function GraphPanel({
  focusId,
  neighborhood,
  loading,
  onFocus,
  onExpandExternal,
  emptyHint,
}: Props) {
  const hostRef = useRef<HTMLDivElement>(null);
  const fgRef = useRef<any>(null);
  const [size, setSize] = useState({ w: 0, h: 0 });
  const [tick, setTick] = useState(0);

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

  useEffect(() => {
    if (!neighborhood) return;
    const { addedNodes, addedLinks } = mergeNeighborhood(neighborhood, focusId);
    setTick((t) => t + 1);
    const fg = fgRef.current;
    if (fg && (addedNodes > 0 || addedLinks > 0)) {
      try {
        fg.d3ReheatSimulation?.();
      } catch {
        /* ignore */
      }
    }
  }, [neighborhood, focusId]);

  // Keep focus highlight without rewriting node objects.
  useEffect(() => {
    getGraphSession().focusId = focusId;
    setTick((t) => t + 1);
  }, [focusId]);

  const graphData = useMemo(() => {
    void tick;
    return snapshotGraphData();
  }, [tick]);

  const onNodeClick = useCallback(
    (node: { id: string; expandable?: boolean; external?: boolean }) => {
      const external = node.external || isExternalId(node.id);
      if (external && node.expandable !== false && onExpandExternal) {
        onExpandExternal(node.id);
        markExpanded(node.id);
        setTick((t) => t + 1);
        return;
      }
      onFocus(node.id);
    },
    [onFocus, onExpandExternal]
  );

  if (graphData.nodes.length === 0) {
    return (
      <div ref={hostRef} className="graph-canvas-host relative h-full min-h-64">
        <div className="p-4 text-sm text-base-content/60 flex items-center gap-2">
          {loading ? <span className="loading loading-spinner loading-xs" /> : null}
          {loading
            ? "Discovering relations…"
            : emptyHint ??
              "Focus a file or symbol to expand the relation graph (lazy, incomplete)."}
        </div>
      </div>
    );
  }

  const sess = getGraphSession();

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
          <span className="badge badge-ghost badge-sm gap-1">
            <span className="loading loading-spinner loading-xs" />
            expanding
          </span>
        ) : null}
        <span className="badge badge-ghost badge-sm opacity-70">
          click external to expand
        </span>
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
