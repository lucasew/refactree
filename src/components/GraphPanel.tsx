import React, { useMemo, useCallback, useRef } from "react";
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
  neighborhood: Neighborhood;
  onFocus: (ref: string) => void;
};

type FGNode = { id: string; name: string; kind: string };
type FGLink = { source: string; target: string; kind: string };

export function GraphPanel({ neighborhood, onFocus }: Props) {
  const fgRef = useRef<any>(null);

  const data = useMemo(() => {
    if (!neighborhood) {
      return { nodes: [] as FGNode[], links: [] as FGLink[] };
    }
    const nodes: FGNode[] = [];
    const seen = new Set<string>();
    for (const n of neighborhood.nodes) {
      if (!n || seen.has(n.id)) continue;
      seen.add(n.id);
      nodes.push({ id: n.id, name: n.label || n.id, kind: n.kind });
    }
    const links: FGLink[] = [];
    for (const e of neighborhood.edges ?? []) {
      if (!e) continue;
      if (!seen.has(e.from) || !seen.has(e.to)) continue;
      links.push({ source: e.from, target: e.to, kind: e.kind });
    }
    return { nodes, links };
  }, [neighborhood]);

  const onNodeClick = useCallback(
    (node: FGNode) => {
      onFocus(node.id);
    },
    [onFocus]
  );

  if (!neighborhood || data.nodes.length === 0) {
    return (
      <p className="graph-empty">
        Focus a file or symbol to expand the relation graph (lazy, incomplete).
      </p>
    );
  }

  return (
    <>
      {neighborhood.incomplete ? (
        <p className="muted" style={{ position: "absolute", zIndex: 1, margin: "0.5rem", fontSize: 12 }}>
          incomplete neighborhood
        </p>
      ) : null}
      <ForceGraph2D
        ref={fgRef}
        graphData={data}
        nodeId="id"
        nodeLabel="name"
        nodeCanvasObject={(node: any, ctx, globalScale) => {
          const label = node.name || node.id;
          const fontSize = 12 / globalScale;
          const r = node.kind === "ATOM" ? 4 : node.kind === "MODULE" ? 7 : 5;
          ctx.beginPath();
          ctx.arc(node.x, node.y, r, 0, 2 * Math.PI, false);
          ctx.fillStyle =
            node.id === neighborhood.focus.id
              ? "#e8a838"
              : node.kind === "ATOM"
                ? "#6bcf8e"
                : node.kind === "MODULE"
                  ? "#7aa2f7"
                  : "#a0a0a0";
          ctx.fill();
          ctx.font = `${fontSize}px sans-serif`;
          ctx.textAlign = "center";
          ctx.textBaseline = "top";
          ctx.fillStyle = "#e8e0d0";
          ctx.fillText(label, node.x, node.y + r + 1);
        }}
        nodePointerAreaPaint={(node: any, color, ctx) => {
          ctx.beginPath();
          ctx.arc(node.x, node.y, 8, 0, 2 * Math.PI, false);
          ctx.fillStyle = color;
          ctx.fill();
        }}
        linkColor={(l: any) =>
          l.kind === "IMPORTS" ? "#5a6a8a" : l.kind === "USED_BY" ? "#8a6a5a" : "#6a8a5a"
        }
        onNodeClick={onNodeClick}
        cooldownTicks={80}
        linkDirectionalArrowLength={3}
        linkDirectionalArrowRelPos={1}
        backgroundColor="#1a1814"
      />
    </>
  );
}
