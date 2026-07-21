/**
 * Usage- + density-weighted layout forces for react-force-graph-2d / d3-force.
 *
 * Criteria:
 *  1. Usage (indegree) — heavily used → center; unused → rim
 *  2. Node density — crowded regions weaken center pull and get soft
 *     spatial repulsion so the core does not collapse into one blob
 */

export type DegreeMap = Map<string, number>;

type SimNode = { id?: string; x?: number; y?: number; vx?: number; vy?: number };

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
 * Local spatial density: count of other nodes within radius (soft).
 * Returns 0…1-ish scores (raw counts normalized by max observed that tick).
 */
function localDensities(
  nodes: SimNode[],
  radius: number
): { dens: Float64Array; maxD: number } {
  const n = nodes.length;
  const dens = new Float64Array(n);
  const r2 = radius * radius;
  for (let i = 0; i < n; i++) {
    const a = nodes[i];
    if (a.x == null || a.y == null) continue;
    let d = 0;
    for (let j = 0; j < n; j++) {
      if (i === j) continue;
      const b = nodes[j];
      if (b.x == null || b.y == null) continue;
      const dx = b.x - a.x;
      const dy = b.y - a.y;
      const dist2 = dx * dx + dy * dy;
      if (dist2 >= r2) continue;
      // soft kernel: 1 at contact, 0 at radius
      d += 1 - Math.sqrt(dist2) / radius;
    }
    dens[i] = d;
  }
  let maxD = 0;
  for (let i = 0; i < n; i++) if (dens[i] > maxD) maxD = dens[i];
  return { dens, maxD: maxD || 1 };
}

/**
 * d3-force: pull toward (0,0) from usage, attenuated by local node density.
 * Sparse hubs still dive to center; crowded hubs keep more spacing.
 */
export function forceUsageGravity(
  getUsage: (id: string) => number,
  opts?: {
    /** Floor pull so simulation doesn't float forever (default 0.008). */
    base?: number;
    /** Extra pull per incoming use, capped (default 0.05). */
    perUse?: number;
    /** Max usage for scaling (default 24). */
    maxUsage?: number;
    /** Neighborhood radius for density sampling (default 52). */
    densityRadius?: number;
    /**
     * How much density weakens center pull (0–1, default 0.65).
     * At max local density, gravity strength is scaled by (1 - densityWeight).
     */
    densityWeight?: number;
  }
) {
  const base = opts?.base ?? 0.008;
  const perUse = opts?.perUse ?? 0.05;
  const maxUsage = opts?.maxUsage ?? 24;
  const densityRadius = opts?.densityRadius ?? 52;
  const densityWeight = opts?.densityWeight ?? 0.65;
  let nodes: SimNode[] = [];

  function force(alpha: number) {
    const { dens, maxD } = localDensities(nodes, densityRadius);
    for (let i = 0; i < nodes.length; i++) {
      const node = nodes[i];
      if (node.x == null || node.y == null) continue;
      const u = Math.min(getUsage(node.id ?? ""), maxUsage);
      const densNorm = dens[i] / maxD; // 0 sparse → 1 densest this tick
      // usage pulls in; density eases pull so crowded cores open up
      const atten = 1 - densityWeight * densNorm;
      const k = alpha * (base + u * perUse) * Math.max(0.15, atten);
      node.vx = (node.vx ?? 0) - (node.x ?? 0) * k;
      node.vy = (node.vy ?? 0) - (node.y ?? 0) * k;
    }
  }

  force.initialize = (initNodes: SimNode[]) => {
    nodes = initNodes;
  };

  return force;
}

/**
 * Soft radial target: unused → outer ring, heavily used → near origin.
 * Dense neighborhoods get a slightly larger preferred radius (spread).
 */
export function forceUsageRadial(
  getUsage: (id: string) => number,
  opts?: {
    /** Radius for unused nodes (default 180). */
    outer?: number;
    /** Radius for heavily used nodes (default 12). */
    inner?: number;
    maxUsage?: number;
    strength?: number;
    densityRadius?: number;
    /** Extra preferred radius at max density (default 28). */
    densitySpread?: number;
  }
) {
  const outer = opts?.outer ?? 180;
  const inner = opts?.inner ?? 12;
  const maxUsage = opts?.maxUsage ?? 24;
  const strength = opts?.strength ?? 0.12;
  const densityRadius = opts?.densityRadius ?? 52;
  const densitySpread = opts?.densitySpread ?? 28;
  let nodes: SimNode[] = [];

  function force(alpha: number) {
    const { dens, maxD } = localDensities(nodes, densityRadius);
    for (let i = 0; i < nodes.length; i++) {
      const node = nodes[i];
      if (node.x == null || node.y == null) continue;
      const u = Math.min(getUsage(node.id ?? ""), maxUsage);
      const t = u / maxUsage; // 0 = unused → outer; 1 = hub → inner
      const densNorm = dens[i] / maxD;
      const targetR = outer + (inner - outer) * t + densitySpread * densNorm;
      const x = node.x;
      const y = node.y;
      const r = Math.hypot(x, y) || 1e-6;
      const delta = (targetR - r) * strength * alpha;
      node.vx = (node.vx ?? 0) + (x / r) * delta;
      node.vy = (node.vy ?? 0) + (y / r) * delta;
    }
  }

  force.initialize = (initNodes: SimNode[]) => {
    nodes = initNodes;
  };

  return force;
}

/**
 * Soft spatial repulsion from local density peaks (pairwise within radius).
 * Complements charge: stronger only when nodes actually overlap neighborhoods.
 */
export function forceNodeDensity(opts?: {
  /** Interaction radius (default 48). */
  radius?: number;
  strength?: number;
}) {
  const radius = opts?.radius ?? 48;
  const strength = opts?.strength ?? 0.4;
  const r2 = radius * radius;
  let nodes: SimNode[] = [];

  function force(alpha: number) {
    const n = nodes.length;
    const gx = new Float64Array(n);
    const gy = new Float64Array(n);
    for (let i = 0; i < n; i++) {
      const a = nodes[i];
      if (a.x == null || a.y == null) continue;
      for (let j = i + 1; j < n; j++) {
        const b = nodes[j];
        if (b.x == null || b.y == null) continue;
        const dx = b.x - a.x;
        const dy = b.y - a.y;
        const d2 = dx * dx + dy * dy;
        if (d2 > r2 || d2 < 1e-10) continue;
        const d = Math.sqrt(d2);
        // unit separation * falloff
        const f = ((1 - d / radius) / d) * strength * alpha;
        gx[i] -= dx * f;
        gy[i] -= dy * f;
        gx[j] += dx * f;
        gy[j] += dy * f;
      }
    }
    for (let i = 0; i < n; i++) {
      const node = nodes[i];
      node.vx = (node.vx ?? 0) + gx[i];
      node.vy = (node.vy ?? 0) + gy[i];
    }
  }

  force.initialize = (initNodes: SimNode[]) => {
    nodes = initNodes;
  };

  return force;
}

/** Node radius scale from usage score (for drawing). */
export function degreeRadiusBoost(usage: number, base: number): number {
  return base + Math.min(usage, 16) * 0.35;
}
