/**
 * Usage-weighted layout forces for react-force-graph-2d / d3-force.
 *
 * High-degree nodes (more incident edges = more "used") get a stronger pull
 * toward the origin; low-degree nodes drift outward.
 */

export type DegreeMap = Map<string, number>;

/** Count undirected degree from session-style links {source, target}. */
export function computeDegrees(
  links: ReadonlyArray<{ source: string | { id?: string }; target: string | { id?: string } }>
): DegreeMap {
  const deg = new Map<string, number>();
  const idOf = (x: string | { id?: string }) =>
    typeof x === "string" ? x : x?.id ?? "";
  for (const l of links) {
    const a = idOf(l.source);
    const b = idOf(l.target);
    if (!a || !b || a === b) continue;
    deg.set(a, (deg.get(a) ?? 0) + 1);
    deg.set(b, (deg.get(b) ?? 0) + 1);
  }
  return deg;
}

/**
 * d3-force custom force: pull toward (0,0) proportional to usage degree.
 * strength at degree 0 is near zero; high-degree nodes stay central.
 */
export function forceUsageGravity(getDegree: (id: string) => number, opts?: {
  /** Base center pull (default 0.02). */
  base?: number;
  /** Extra pull per edge, capped (default 0.035). */
  perEdge?: number;
  /** Max degree used for strength scaling (default 24). */
  maxDegree?: number;
}) {
  const base = opts?.base ?? 0.02;
  const perEdge = opts?.perEdge ?? 0.035;
  const maxDegree = opts?.maxDegree ?? 24;
  let nodes: Array<{ id?: string; x?: number; y?: number; vx?: number; vy?: number }> = [];

  function force(alpha: number) {
    for (const node of nodes) {
      if (node.x == null || node.y == null) continue;
      const id = node.id ?? "";
      const d = Math.min(getDegree(id), maxDegree);
      // Leaf nodes (0–1): weak gravity → periphery
      // Hubs: strong gravity → center
      const k = alpha * (base + d * perEdge);
      node.vx = (node.vx ?? 0) - (node.x ?? 0) * k;
      node.vy = (node.vy ?? 0) - (node.y ?? 0) * k;
    }
  }

  force.initialize = (initNodes: typeof nodes) => {
    nodes = initNodes;
  };

  return force;
}

/** Soft radial bias: inverse of degree → preferred distance from origin. */
export function forceUsageRadial(getDegree: (id: string) => number, opts?: {
  /** Radius for degree-0 nodes (default 180). */
  outer?: number;
  /** Radius for high-degree nodes (default 12). */
  inner?: number;
  /** Max degree for scaling (default 24). */
  maxDegree?: number;
  strength?: number;
}) {
  const outer = opts?.outer ?? 180;
  const inner = opts?.inner ?? 12;
  const maxDegree = opts?.maxDegree ?? 24;
  const strength = opts?.strength ?? 0.08;
  let nodes: Array<{ id?: string; x?: number; y?: number; vx?: number; vy?: number }> = [];

  function force(alpha: number) {
    for (const node of nodes) {
      if (node.x == null || node.y == null) continue;
      const d = Math.min(getDegree(node.id ?? ""), maxDegree);
      const t = d / maxDegree; // 0 = unused, 1 = hub
      const targetR = outer + (inner - outer) * t;
      const x = node.x;
      const y = node.y;
      const r = Math.hypot(x, y) || 1e-6;
      // push/pull along radius toward targetR
      const delta = (targetR - r) * strength * alpha;
      node.vx = (node.vx ?? 0) + (x / r) * delta;
      node.vy = (node.vy ?? 0) + (y / r) * delta;
    }
  }

  force.initialize = (initNodes: typeof nodes) => {
    nodes = initNodes;
  };

  return force;
}

/** Node radius scale from degree (for drawing). */
export function degreeRadiusBoost(degree: number, base: number): number {
  return base + Math.min(degree, 16) * 0.35;
}
