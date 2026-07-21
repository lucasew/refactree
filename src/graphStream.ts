import {
  getGraphSession,
  isExternalId,
  linkKey,
  markExpanded,
  type IncomingEdge,
  type IncomingNode,
} from "./graphSession";

export type StreamHandlers = {
  onEvent?: () => void;
  onDone?: () => void;
  onError?: (err: Error) => void;
};

const pendingNodeIds = new Set<string>();
let hydrateTimer: ReturnType<typeof setTimeout> | null = null;
let hydrateInFlight = false;

function stubFromId(id: string): IncomingNode {
  const external = isExternalId(id);
  let kind = "MODULE";
  let label = id;
  if (id.includes("::")) {
    kind = "ATOM";
    label = id.split("::").pop() || id;
  } else if (id.startsWith("path:") && id.includes(".")) {
    kind = "FILE";
    label = id.replace(/^path:(\.\/)?/, "").split("/").pop() || id;
  } else if (id.startsWith("path:")) {
    kind = "MODULE";
    label = id.replace(/^path:(\.\/)?/, "") || "project";
  } else if (id.includes(":")) {
    label = id.slice(id.indexOf(":") + 1) || id;
  }
  return {
    id,
    kind,
    label,
    external,
    expandable: external,
  };
}

function applyNode(n: IncomingNode, markResolved = true) {
  const s = getGraphSession();
  const existing = s.nodes.get(n.id);
  if (existing) {
    existing.name = n.label || existing.name || n.id;
    existing.kind = n.kind || existing.kind;
    if (n.external != null) existing.external = !!n.external;
    if (n.expandable != null) existing.expandable = !!n.expandable;
    if (markResolved) (existing as any).resolved = true;
  } else {
    s.nodes.set(n.id, {
      id: n.id,
      name: n.label || n.id,
      kind: n.kind,
      external: !!n.external,
      expandable: !!n.expandable,
      ...(markResolved ? { resolved: true } : { resolved: false }),
    } as any);
  }
}

function ensureStub(id: string) {
  if (!id || getGraphSession().nodes.has(id)) return;
  applyNode(stubFromId(id), false);
  pendingNodeIds.add(id);
  scheduleHydrate();
}

function applyEdge(e: IncomingEdge) {
  if (!e.from || !e.to) return;
  const s = getGraphSession();
  const k = linkKey(e.from, e.to, e.kind);
  if (!s.links.has(k)) {
    s.links.set(k, { source: e.from, target: e.to, kind: e.kind });
  }
  ensureStub(e.from);
  ensureStub(e.to);
}

function scheduleHydrate() {
  if (hydrateTimer) return;
  hydrateTimer = setTimeout(() => {
    hydrateTimer = null;
    void hydratePendingNodes();
  }, 40);
}

async function hydratePendingNodes() {
  if (hydrateInFlight || pendingNodeIds.size === 0) return;
  hydrateInFlight = true;
  const ids = Array.from(pendingNodeIds).slice(0, 64);
  for (const id of ids) pendingNodeIds.delete(id);
  try {
    const res = await fetch("/graphql", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        query: `query HydrateNodes($ids: [ID!]!) {
          nodes(ids: $ids) {
            id kind label parentId external expandable
          }
        }`,
        variables: { ids },
      }),
    });
    const json = await res.json();
    const nodes = json?.data?.nodes as IncomingNode[] | undefined;
    if (nodes) {
      for (const n of nodes) {
        if (n?.id) applyNode(n, true);
      }
    }
  } catch {
    // leave stubs; may retry later if re-queued
    for (const id of ids) pendingNodeIds.add(id);
  } finally {
    hydrateInFlight = false;
    if (pendingNodeIds.size > 0) scheduleHydrate();
  }
}

function applyEvent(ev: any, focusFallback: string) {
  const s = getGraphSession();
  if (ev.incomplete != null) s.incomplete = !!ev.incomplete;

  switch (ev.type) {
    case "focus":
      if (ev.node) {
        applyNode(ev.node, true);
        s.focusId = ev.node.id;
        // parent of focus may be useful; query on demand
        if (ev.node.parentId) ensureStub(ev.node.parentId);
      }
      break;
    case "node":
      // legacy; treat as on-demand hydration
      if (ev.node) applyNode(ev.node, true);
      break;
    case "edge":
      if (ev.edge) {
        applyEdge({
          from: ev.edge.from,
          to: ev.edge.to,
          kind: ev.edge.kind,
        });
      }
      break;
    case "done":
      if (ev.incomplete != null) s.incomplete = !!ev.incomplete;
      if (!s.focusId && focusFallback) s.focusId = focusFallback;
      // final hydrate pass
      scheduleHydrate();
      break;
    case "error":
      throw new Error(ev.message || "graph stream error");
  }
}

export async function streamGraph(
  opts: { ref?: string; mode?: "project" | "neighborhood" },
  handlers: StreamHandlers = {},
  signal?: AbortSignal
): Promise<void> {
  const params = new URLSearchParams();
  if (opts.mode === "project") {
    params.set("mode", "project");
  } else {
    params.set("ref", opts.ref || "path:./");
  }
  const url = `/api/graph/stream?${params.toString()}`;
  const res = await fetch(url, {
    headers: { Accept: "text/event-stream" },
    signal,
  });
  if (!res.ok || !res.body) {
    throw new Error(`graph stream failed: HTTP ${res.status}`);
  }

  const reader = res.body.getReader();
  const dec = new TextDecoder();
  let buf = "";
  const focusFallback = opts.ref || "path:./";

  const flushEvent = (type: string, data: string) => {
    if (!data) return;
    const ev = JSON.parse(data);
    if (type && type !== "message") ev.type = type;
    applyEvent(ev, focusFallback);
    handlers.onEvent?.();
    if (ev.type === "done") handlers.onDone?.();
  };

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += dec.decode(value, { stream: true });
      for (;;) {
        const sep = buf.indexOf("\n\n");
        if (sep < 0) break;
        const block = buf.slice(0, sep);
        buf = buf.slice(sep + 2);
        let et = "message";
        const dataLines: string[] = [];
        for (const line of block.split("\n")) {
          if (line.startsWith("event:")) et = line.slice(6).trim();
          else if (line.startsWith("data:")) dataLines.push(line.slice(5).trim());
        }
        flushEvent(et, dataLines.join("\n"));
      }
    }
  } catch (e) {
    if ((e as Error).name === "AbortError") return;
    handlers.onError?.(e as Error);
    throw e;
  }
}

export async function streamExpandExternal(
  ref: string,
  handlers: StreamHandlers = {},
  signal?: AbortSignal
): Promise<void> {
  await streamGraph({ ref, mode: "neighborhood" }, handlers, signal);
  markExpanded(ref);
}
