import {
  formatGraphLabel,
  graphScopeId,
  normalizeRef,
  moduleScopeId,
  type GraphViewMode,
} from "./routes";

/**
 * Session-wide graph facts + dual projections for module / atom views.
 *
 * Facts (nodes/links) are always full graph ids. Projections are maintained
 * incrementally on upsert so snapshotGraphData is O(mode size), not a full
 * rescan of the session every paint.
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
  /** Full fact store (modules + atoms). */
  nodes: Map<string, FGNode>;
  links: Map<FGLinkKey, FGLinkFact>;
  /** Incremental module projection (stable node objects). */
  moduleNodes: Map<string, FGNode>;
  moduleLinks: Map<FGLinkKey, FGLinkFact>;
  /** Incremental atom projection (subset of nodes + atom↔atom links). */
  atomNodes: Map<string, FGNode>;
  atomLinks: Map<FGLinkKey, FGLinkFact>;
  expanded: Set<string>;
  focusId: string;
  incomplete: boolean;
  /** Bumps when a projection changes (optional dirty signal). */
  version: number;
};

const session: GraphSession = {
  nodes: new Map(),
  links: new Map(),
  moduleNodes: new Map(),
  moduleLinks: new Map(),
  atomNodes: new Map(),
  atomLinks: new Map(),
  expanded: new Set(),
  focusId: "",
  incomplete: true,
  version: 0,
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

export function isExternalId(id: string): boolean {
  const i = id.indexOf(":");
  if (i <= 0) return false;
  const prov = id.slice(0, i).toLowerCase();
  return prov !== "path";
}

export function isAtomId(id: string): boolean {
  return (id ?? "").includes("::");
}

export function isModuleId(id: string): boolean {
  const s = id ?? "";
  return s !== "" && !s.includes("::");
}

function bumpVersion() {
  session.version++;
}

/** Ensure module projection has a stable node for moduleScopeId(id). */
function ensureModuleNode(fromFact: FGNode): FGNode {
  const pid = moduleScopeId(fromFact.id);
  let p = session.moduleNodes.get(pid);
  if (!p) {
    const seed = session.nodes.get(pid);
    if (seed && isModuleId(seed.id)) {
      p = seed;
    } else {
      p = {
        id: pid,
        name: formatGraphLabel(pid, "module"),
        kind: "MODULE",
        external: !!fromFact.external,
        expandable: !!fromFact.expandable && !!fromFact.external,
        language: fromFact.language,
      };
    }
    session.moduleNodes.set(pid, p);
  }
  p.id = pid;
  p.name = formatGraphLabel(pid, "module");
  p.kind = "MODULE";
  if (fromFact.external) p.external = true;
  if (fromFact.expandable && fromFact.external) p.expandable = true;
  if (fromFact.language && !p.language) p.language = fromFact.language;
  return p;
}

/**
 * Upsert a fact node and maintain module/atom projections incrementally.
 */
export function upsertSessionNode(partial: {
  id: string;
  kind?: string;
  name?: string;
  external?: boolean;
  expandable?: boolean;
  language?: string;
}): FGNode {
  const id = graphScopeId(partial.id);
  const isAtom = isAtomId(id);
  const existing = session.nodes.get(id);
  if (existing) {
    if (partial.name) existing.name = partial.name;
    else existing.name = formatGraphLabel(id, "atom");
    if (partial.kind) existing.kind = isAtom ? partial.kind : "MODULE";
    else if (!isAtom) existing.kind = "MODULE";
    if (partial.external != null) existing.external = partial.external;
    if (partial.expandable != null) existing.expandable = partial.expandable;
    if (partial.language) existing.language = partial.language;
    ensureModuleNode(existing);
    if (isAtom) {
      session.atomNodes.set(id, existing);
    }
    bumpVersion();
    return existing;
  }
  const node: FGNode = {
    id,
    name: partial.name || formatGraphLabel(id, "atom"),
    kind: isAtom ? partial.kind || "ATOM" : "MODULE",
    external: !!partial.external,
    expandable: !!partial.expandable,
    language: partial.language,
  };
  session.nodes.set(id, node);
  ensureModuleNode(node);
  if (isAtom) {
    session.atomNodes.set(id, node);
  }
  bumpVersion();
  return node;
}

/**
 * Upsert a fact edge and maintain module/atom link projections.
 */
export function upsertSessionLink(fromRaw: string, toRaw: string, kind: string): boolean {
  const from = graphScopeId(fromRaw);
  const to = graphScopeId(toRaw);
  if (!from || !to || from === to) return false;

  const k = linkKey(from, to, kind);
  let added = false;
  if (!session.links.has(k)) {
    session.links.set(k, { source: from, target: to, kind });
    added = true;
  }

  // Ensure endpoint facts exist (stubs).
  if (!session.nodes.has(from)) {
    upsertSessionNode({ id: from, kind: isAtomId(from) ? "ATOM" : "MODULE" });
  } else {
    ensureModuleNode(session.nodes.get(from)!);
  }
  if (!session.nodes.has(to)) {
    upsertSessionNode({ id: to, kind: isAtomId(to) ? "ATOM" : "MODULE" });
  } else {
    ensureModuleNode(session.nodes.get(to)!);
  }

  // Module projection: only directed IMPORTS between modules
  // (A imports B → A→B). Do not rewire USES/USED_BY — those blur direction.
  if (kind === "IMPORTS") {
    const pf = moduleScopeId(from);
    const pt = moduleScopeId(to);
    if (pf && pt && pf !== pt && isModuleId(pf) && isModuleId(pt)) {
      const pk = linkKey(pf, pt, kind);
      if (!session.moduleLinks.has(pk)) {
        session.moduleLinks.set(pk, { source: pf, target: pt, kind: "IMPORTS" });
        added = true;
      }
    }
  }

  // Reference projection: only atom ↔ atom.
  if (isAtomId(from) && isAtomId(to)) {
    const rk = linkKey(from, to, kind);
    if (!session.atomLinks.has(rk)) {
      session.atomLinks.set(rk, { source: from, target: to, kind });
      added = true;
    }
    const a = session.nodes.get(from);
    const b = session.nodes.get(to);
    if (a) session.atomNodes.set(from, a);
    if (b) session.atomNodes.set(to, b);
  }

  if (added) bumpVersion();
  return added;
}

/** Merge a neighborhood into the session, reusing node objects for stable positions. */
export function mergeNeighborhood(
  nb: IncomingNeighborhood,
  focusFallback = ""
): { addedNodes: number; addedLinks: number } {
  if (!nb) return { addedNodes: 0, addedLinks: 0 };
  let addedNodes = 0;
  let addedLinks = 0;

  for (const n of nb.nodes ?? []) {
    if (!n?.id) continue;
    const before = session.nodes.has(graphScopeId(n.id));
    upsertSessionNode({
      id: n.id,
      kind: n.kind,
      name: formatGraphLabel(graphScopeId(n.id), "atom"),
      external: n.external ?? undefined,
      expandable: n.expandable ?? undefined,
      language: n.language ?? undefined,
    });
    if (!before) addedNodes++;
  }

  for (const e of nb.edges ?? []) {
    if (!e?.from || !e?.to) continue;
    if (upsertSessionLink(e.from, e.to, e.kind)) addedLinks++;
  }

  session.incomplete = nb.incomplete;
  const focusRaw = nb.focus?.id ?? focusFallback;
  session.focusId = focusRaw ? graphScopeId(focusRaw) : session.focusId;
  return { addedNodes, addedLinks };
}

export function markExpanded(id: string) {
  const nid = graphScopeId(id);
  session.expanded.add(nid);
  const n = session.nodes.get(nid);
  if (n) n.expandable = false;
  const p = session.moduleNodes.get(moduleScopeId(nid));
  if (p) p.expandable = false;
}

/**
 * O(projection size) snapshot — projections are maintained on upsert.
 */
export function snapshotGraphData(
  mode: GraphViewMode = "atom"
): { nodes: FGNode[]; links: { source: string; target: string; kind: string }[] } {
  if (mode === "module") {
    const nodes = Array.from(session.moduleNodes.values());
    const links: { source: string; target: string; kind: string }[] = [];
    for (const l of session.moduleLinks.values()) {
      links.push({ source: l.source, target: l.target, kind: l.kind });
    }
    return { nodes, links };
  }
  const nodes = Array.from(session.atomNodes.values());
  const links: { source: string; target: string; kind: string }[] = [];
  for (const l of session.atomLinks.values()) {
    links.push({ source: l.source, target: l.target, kind: l.kind });
  }
  return { nodes, links };
}

/** Focus id for the current view. */
export function viewFocusId(focusId: string, mode: GraphViewMode): string {
  const id = normalizeRef(focusId || "");
  if (!id) return id;
  if (mode === "module") return moduleScopeId(id);
  return isAtomId(id) ? graphScopeId(id) : "";
}
