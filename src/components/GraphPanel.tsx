import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import ForceGraph2D from "react-force-graph-2d";

type GNode = {
  id: string;
  kind: string;
  label: string;
  parentId?: string | null;
};

type GEdge = {
  from: string;
  to: string;
  kind: string;
};

type Neighborhood = {
  incomplete: boolean;
  focus: GNode;
  nodes: ReadonlyArray<GNode | null | undefined>;
  edges: ReadonlyArray<GEdge | null | undefined>;
} | null | undefined;

type Props = {
  focusId: string;
  neighborhood: Neighborhood;
  loading?: boolean;
  onFocus: (ref: string) => void;
};

type FGNode = { id: string; name: string; kind: string; x?: number; y?: number };
type FGLink = { source: string; target: string; kind: string };

const BG = "#1a1814";

export function GraphPanel({ focusId, neighborhood, loading, onFocus }: Props) {
  const hostRef = useRef<HTMLDivElement>(null);
  const fgRef = useRef<any>(null);
  const [size, setSize] = useState({ w: 0, h: 0 });

  // Immutable facts only (string endpoints). Force-graph mutates copies, not this store.
  const [merged, setMerged] = useState<{
    nodes: Map<string, FGNode>;
    links: Map<string, { source: string; target: string; kind: string }>;
    incomplete: boolean;
    focusId: string;
  }>(() => ({
    nodes: new Map(),
    links: new Map(),
    incomplete: true,
    focusId: "",
  }));

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
    setMerged((prev) => {
      const nodes = new Map(prev.nodes);
      const links = new Map(prev.links);
      for (const n of neighborhood.nodes ?? []) {
        if (!n) continue;
        const existing = nodes.get(n.id);
        nodes.set(n.id, {
          id: n.id,
          name: n.label || n.id,
          kind: n.kind,
          x: existing?.x,
          y: existing?.y,
        });
      }
      for (const e of neighborhood.edges ?? []) {
        if (!e?.from || !e?.to) continue;
        const key = `${e.from}\0${e.to}\0${e.kind}`;
        links.set(key, { source: e.from, target: e.to, kind: e.kind });
      }
      return {
        nodes,
        links,
        incomplete: neighborhood.incomplete,
        focusId: neighborhood.focus?.id ?? focusId,
      };
    });
  }, [neighborhood, focusId]);

  useEffect(() => {
    const fg = fgRef.current;
    if (!fg) return;
    try {
      fg.d3ReheatSimulation?.();
    } catch {
      /* ignore */
    }
  }, [merged.focusId, merged.nodes.size, merged.links.size]);

  // Fresh objects every time so force-graph cannot corrupt the Map via in-place mutation.
  const graphData = useMemo(() => {
    const nodes = Array.from(merged.nodes.values()).map((n) => ({ ...n }));
    const nodeIds = new Set(nodes.map((n) => n.id));
    const links: FGLink[] = [];
    for (const l of merged.links.values()) {
      if (!nodeIds.has(l.source) || !nodeIds.has(l.target)) continue;
      links.push({ source: l.source, target: l.target, kind: l.kind });
    }
    return { nodes, links };
  }, [merged]);

  const onNodeClick = useCallback(
    (node: FGNode) => {
      onFocus(node.id);
    },
    [onFocus]
  );

  const onNodeDragEnd = useCallback((node: FGNode) => {
    setMerged((prev) => {
      const nodes = new Map(prev.nodes);
      const cur = nodes.get(node.id);
      if (cur) {
        nodes.set(node.id, { ...cur, x: node.x, y: node.y });
      }
      return { ...prev, nodes };
    });
  }, []);

  if (graphData.nodes.length === 0) {
    return (
      <div ref={hostRef} className="graph-canvas-host relative h-full min-h-64">
        <div className="p-4 text-sm text-base-content/60 flex items-center gap-2">
          {loading ? <span className="loading loading-spinner loading-xs" /> : null}
          {loading
            ? "Discovering relations…"
            : "Focus a file or symbol to expand the relation graph (lazy, incomplete)."}
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
        {merged.incomplete ? (
          <span className="badge badge-ghost badge-sm">incomplete</span>
        ) : null}
        {loading ? (
          <span className="badge badge-ghost badge-sm gap-1">
            <span className="loading loading-spinner loading-xs" />
            expanding
          </span>
        ) : null}
      </div>
      {size.w > 0 && size.h > 0 ? (
        <ForceGraph2D
          ref={fgRef}
          width={size.w}
          height={size.h}
          graphData={graphData}
          nodeId="id"
          nodeLabel="name"
          backgroundColor={BG}
          linkWidth={1.25}
          linkColor={(l: any) => {
            const kind = l.kind as string;
            if (kind === "IMPORTS") return "#7a8aaa";
            if (kind === "USED_BY") return "#aa8a7a";
            return "#8aaa7a"; // USES
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
            ctx.fillStyle =
              node.id === merged.focusId
                ? "#e8a838"
                : node.kind === "ATOM"
                  ? "#6bcf8e"
                  : node.kind === "MODULE"
                    ? "#7aa2f7"
                    : "#a0a0a0";
            ctx.fill();
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
          onNodeDragEnd={onNodeDragEnd}
          cooldownTicks={100}
          autoPauseRedraw={false}
          enableNodeDrag={true}
        />
      ) : null}
    </div>
  );
}
