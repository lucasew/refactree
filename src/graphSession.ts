/**
 * Session-wide graph facts + stable node object identity for force-graph.
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
  nodes: Map<string, FGNode>;
  links: Map<FGLinkKey, FGLinkFact>;
  /** External nodes already expanded via neighborhood. */
  expanded: Set<string>;
  focusId: string;
  incomplete: boolean;
};

const session: GraphSession = {
  nodes: new Map(),
  links: new Map(),
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
    const existing = s.nodes.get(n.id);
    if (existing) {
      existing.name = n.label || n.id;
      existing.kind = n.kind;
      if (n.external != null) existing.external = n.external;
      if (n.expandable != null) existing.expandable = n.expandable;
      if (n.language != null) existing.language = n.language || existing.language;
      // keep x,y,vx,vy
    } else {
      s.nodes.set(n.id, {
        id: n.id,
        name: n.label || n.id,
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
    const k = linkKey(e.from, e.to, e.kind);
    if (!s.links.has(k)) {
      s.links.set(k, { source: e.from, target: e.to, kind: e.kind });
      addedLinks++;
    }
  }

  s.incomplete = nb.incomplete;
  s.focusId = nb.focus?.id ?? focusFallback ?? s.focusId;
  return { addedNodes, addedLinks };
}

export function markExpanded(id: string) {
  session.expanded.add(id);
  const n = session.nodes.get(id);
  if (n) n.expandable = false;
}

export function isExternalId(id: string): boolean {
  const i = id.indexOf(":");
  if (i <= 0) return false;
  const prov = id.slice(0, i).toLowerCase();
  return prov !== "path";
}

export function snapshotGraphData(): { nodes: FGNode[]; links: { source: string; target: string; kind: string }[] } {
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
