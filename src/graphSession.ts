import {
  formatGraphLabel,
  normalizeRef,
  packageScopeId,
  type GraphViewMode,
} from "./routes";

/**
 * Session-wide graph facts + stable node object identity for force-graph.
 * Facts are always reference-level (full path:./pkg::Sym ids).
 * Package view projects a collapsed universe (one node per packageScopeId).
 * Positions live on FGNode objects; force-graph mutates them in place so
 * switching files/pages does not reset layout when we reuse the same objects.
 */

export type FGNode = {
  id: string;
  name: string;
  kind: string;
  external?: boolean;
  expandable?: boolean;
  language?: string;
  x?: number;
  y?: number;
  vx?: number;
  vy?: number;
};

export type FGLinkKey = string;

export type FGLinkFact = {
  source: string;
  target: string;
  kind: string;
};

export type GraphSession = {
  /** Full-reference nodes (reference view universe). */
  nodes: Map<string, FGNode>;
  links: Map<FGLinkKey, FGLinkFact>;
  /** Collapsed package nodes (stable objects for package view layout). */
  packageNodes: Map<string, FGNode>;
  /** External nodes already expanded via neighborhood. */
  expanded: Set<string>;
  focusId: string;
  incomplete: boolean;
};

const session: GraphSession = {
  nodes: new Map(),
  links: new Map(),
  packageNodes: new Map(),
  expanded: new Set(),
  focusId: "",
  incomplete: true,
};

export function getGraphSession(): GraphSession {
  return session;
}

export function linkKey(from: string, to: string, kind: string): FGLinkKey {
  return `${from}\0${to}\0${kind}`;
}

export type IncomingNode = {
  id: string;
  kind: string;
  label: string;
  parentId?: string | null;
  external?: boolean | null;
  expandable?: boolean | null;
  language?: string | null;
};

export type IncomingEdge = {
  from: string;
  to: string;
  kind: string;
};

export type IncomingNeighborhood = {
  incomplete: boolean;
  focus?: { id: string } | null;
  nodes: ReadonlyArray<IncomingNode | null | undefined>;
  edges: ReadonlyArray<IncomingEdge | null | undefined>;
} | null | undefined;

/** Merge a neighborhood into the session, reusing node objects for stable positions. */
export function mergeNeighborhood(
  nb: IncomingNeighborhood,
  focusFallback = ""
): { addedNodes: number; addedLinks: number } {
  if (!nb) return { addedNodes: 0, addedLinks: 0 };
  let addedNodes = 0;
  let addedLinks = 0;
  const s = session;

  for (const n of nb.nodes ?? []) {
    if (!n?.id) continue;
    const id = normalizeRef(n.id);
    const name = formatGraphLabel(id, "reference");
    const existing = s.nodes.get(id);
    if (existing) {
      existing.name = name;
      existing.kind = n.kind;
      if (n.external != null) existing.external = n.external;
      if (n.expandable != null) existing.expandable = n.expandable;
      if (n.language != null) existing.language = n.language || existing.language;
      // keep x,y,vx,vy
    } else {
      s.nodes.set(id, {
        id,
        name,
        kind: n.kind,
        external: !!n.external,
        expandable: !!n.expandable,
        language: n.language || undefined,
      });
      addedNodes++;
    }
  }

  for (const e of nb.edges ?? []) {
    if (!e?.from || !e?.to) continue;
    const from = normalizeRef(e.from);
    const to = normalizeRef(e.to);
    const k = linkKey(from, to, e.kind);
    if (!s.links.has(k)) {
      s.links.set(k, { source: from, target: to, kind: e.kind });
      addedLinks++;
    }
  }

  s.incomplete = nb.incomplete;
  const focusRaw = nb.focus?.id ?? focusFallback;
  s.focusId = focusRaw ? normalizeRef(focusRaw) : s.focusId;
  return { addedNodes, addedLinks };
}

export function markExpanded(id: string) {
  const nid = normalizeRef(id);
  session.expanded.add(nid);
  const n = session.nodes.get(nid);
  if (n) n.expandable = false;
}

export function isExternalId(id: string): boolean {
  const i = id.indexOf(":");
  if (i <= 0) return false;
  const prov = id.slice(0, i).toLowerCase();
  return prov !== "path";
}

/**
 * Project session facts for the canvas.
 * - reference: one node per full ref id (path:./pkg::Sym kept distinct)
 * - package: collapse to packageScopeId (path:./cmd/rft once; edges rewired)
 */
export function snapshotGraphData(
  mode: GraphViewMode = "reference"
): { nodes: FGNode[]; links: { source: string; target: string; kind: string }[] } {
  if (mode === "package") {
    return snapshotPackageGraphData();
  }
  const nodes = Array.from(session.nodes.values());
  const ids = new Set(nodes.map((n) => n.id));
  const links: { source: string; target: string; kind: string }[] = [];
  for (const l of session.links.values()) {
    if (ids.has(l.source) && ids.has(l.target)) {
      // fresh link objects (string ends) so force-graph cannot corrupt the Map
      links.push({ source: l.source, target: l.target, kind: l.kind });
    }
  }
  return { nodes, links };
}

function snapshotPackageGraphData(): {
  nodes: FGNode[];
  links: { source: string; target: string; kind: string }[];
} {
  const seenPkg = new Set<string>();

  for (const n of session.nodes.values()) {
    const pid = packageScopeId(n.id);
    seenPkg.add(pid);
    let p = session.packageNodes.get(pid);
    if (!p) {
      // Prefer reusing the module-level session node object so positions match reference view.
      const seed = session.nodes.get(pid);
      if (seed) {
        p = seed;
      } else {
        p = {
          id: pid,
          name: formatGraphLabel(pid, "package"),
          kind: "MODULE",
          external: !!n.external,
          expandable: !!n.expandable && !!n.external,
          language: n.language,
        };
      }
      session.packageNodes.set(pid, p);
    }
    // Keep package node id/name/kind coherent under collapse.
    p.id = pid;
    p.name = formatGraphLabel(pid, "package");
    p.kind = "MODULE";
    if (n.external) p.external = true;
    if (n.expandable && n.external) p.expandable = true;
    if (n.language && !p.language) p.language = n.language;
  }

  // Drop package placeholders that no longer have members (session shrink rare).
  for (const id of Array.from(session.packageNodes.keys())) {
    if (!seenPkg.has(id)) session.packageNodes.delete(id);
  }

  const nodes = Array.from(session.packageNodes.values()).filter((n) => seenPkg.has(n.id));
  const ids = new Set(nodes.map((n) => n.id));
  const links: { source: string; target: string; kind: string }[] = [];
  const seenLink = new Set<string>();
  for (const l of session.links.values()) {
    const from = packageScopeId(l.source);
    const to = packageScopeId(l.target);
    if (!from || !to || from === to) continue;
    if (!ids.has(from) || !ids.has(to)) continue;
    const k = `${from}\0${to}\0${l.kind}`;
    if (seenLink.has(k)) continue;
    seenLink.add(k);
    links.push({ source: from, target: to, kind: l.kind });
  }
  return { nodes, links };
}

/** Focus id for the current view (package view maps atoms → package). */
export function viewFocusId(focusId: string, mode: GraphViewMode): string {
  const id = normalizeRef(focusId || "");
  if (!id) return id;
  return mode === "package" ? packageScopeId(id) : id;
}
