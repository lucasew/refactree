/**
 * Usage-weighted layout forces for react-force-graph-2d / d3-force.
 *
 * "Used" = how many others point at this node (indegree on IMPORTS/USES).
 * Heavily used → center; unused → rim.
 */

export type DegreeMap = Map<string, number>;

const idOf = (x: string | { id?: string } | null | undefined) =>
  typeof x === "string" ? x : x?.id ?? "";

/**
 * Usage score = indegree (how many edges target this node).
 * IMPORTS A→B means B is used by A; high score → central.
 */
export function computeUsage(
  links: ReadonlyArray<{
    source: string | { id?: string };
    target: string | { id?: string };
    kind?: string;
  }>
): DegreeMap {
  const use = new Map<string, number>();
  for (const l of links) {
    const a = idOf(l.source);
    const b = idOf(l.target);
    if (!a || !b || a === b) continue;
    // USED_BY is reverse polarity if ever emitted: from used → user.
    if (l.kind === "USED_BY") {
      use.set(a, (use.get(a) ?? 0) + 1);
    } else {
      // IMPORTS / USES / default: target is the dependency (used).
      use.set(b, (use.get(b) ?? 0) + 1);
    }
  }
  return use;
}

/** @deprecated use computeUsage — kept name alias for call sites. */
export function computeDegrees(
  links: ReadonlyArray<{ source: string | { id?: string }; target: string | { id?: string }; kind?: string }>
): DegreeMap {
  return computeUsage(links);
}

/**
 * d3-force: pull toward (0,0) proportional to usage (indegree).
 * Unused ≈ no pull; heavily used → strong center gravity.
 */
export function forceUsageGravity(getUsage: (id: string) => number, opts?: {
  /** Floor pull so simulation doesn't float forever (default 0.008). */
  base?: number;
  /** Extra pull per incoming use, capped (default 0.05). */
  perUse?: number;
  /** Max usage for scaling (default 24). */
  maxUsage?: number;
}) {
  const base = opts?.base ?? 0.008;
  const perUse = opts?.perUse ?? 0.05;
  const maxUsage = opts?.maxUsage ?? 24;
  let nodes: Array<{ id?: string; x?: number; y?: number; vx?: number; vy?: number }> = [];

  function force(alpha: number) {
    for (const node of nodes) {
      if (node.x == null || node.y == null) continue;
      const u = Math.min(getUsage(node.id ?? ""), maxUsage);
      // unused: near-zero pull (base only); hubs: strong pull to center
      const k = alpha * (base + u * perUse);
      node.vx = (node.vx ?? 0) - (node.x ?? 0) * k;
      node.vy = (node.vy ?? 0) - (node.y ?? 0) * k;
    }
  }

  force.initialize = (initNodes: typeof nodes) => {
    nodes = initNodes;
  };

  return force;
}

/**
 * Soft radial target: unused → outer ring, heavily used → near origin.
 */
export function forceUsageRadial(getUsage: (id: string) => number, opts?: {
  /** Radius for unused nodes (default 180). */
  outer?: number;
  /** Radius for heavily used nodes (default 12). */
  inner?: number;
  maxUsage?: number;
  strength?: number;
}) {
  const outer = opts?.outer ?? 180;
  const inner = opts?.inner ?? 12;
  const maxUsage = opts?.maxUsage ?? 24;
  const strength = opts?.strength ?? 0.12;
  let nodes: Array<{ id?: string; x?: number; y?: number; vx?: number; vy?: number }> = [];

  function force(alpha: number) {
    for (const node of nodes) {
      if (node.x == null || node.y == null) continue;
      const u = Math.min(getUsage(node.id ?? ""), maxUsage);
      const t = u / maxUsage; // 0 = unused → outer; 1 = hub → inner
      const targetR = outer + (inner - outer) * t;
      const x = node.x;
      const y = node.y;
      const r = Math.hypot(x, y) || 1e-6;
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

/** Node radius scale from usage score (for drawing). */
export function degreeRadiusBoost(usage: number, base: number): number {
  return base + Math.min(usage, 16) * 0.35;
}
