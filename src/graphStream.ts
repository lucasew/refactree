import {
  getGraphSession,
  linkKey,
  markExpanded,
  type IncomingEdge,
  type IncomingNode,
} from "./graphSession";

export type StreamHandlers = {
  onEvent?: () => void; // UI tick after each applied event
  onDone?: () => void;
  onError?: (err: Error) => void;
};

function applyNode(n: IncomingNode) {
  const s = getGraphSession();
  const existing = s.nodes.get(n.id);
  if (existing) {
    existing.name = n.label || n.id;
    existing.kind = n.kind;
    if (n.external != null) existing.external = !!n.external;
    if (n.expandable != null) existing.expandable = !!n.expandable;
  } else {
    s.nodes.set(n.id, {
      id: n.id,
      name: n.label || n.id,
      kind: n.kind,
      external: !!n.external,
      expandable: !!n.expandable,
    });
  }
}

function applyEdge(e: IncomingEdge) {
  if (!e.from || !e.to) return;
  const s = getGraphSession();
  const k = linkKey(e.from, e.to, e.kind);
  if (!s.links.has(k)) {
    s.links.set(k, { source: e.from, target: e.to, kind: e.kind });
  }
  // ensure endpoints exist as stubs
  if (!s.nodes.has(e.from)) {
    s.nodes.set(e.from, {
      id: e.from,
      name: e.from,
      kind: "MODULE",
      external: !e.from.startsWith("path:"),
      expandable: !e.from.startsWith("path:"),
    });
  }
  if (!s.nodes.has(e.to)) {
    s.nodes.set(e.to, {
      id: e.to,
      name: e.to,
      kind: "MODULE",
      external: !e.to.startsWith("path:"),
      expandable: !e.to.startsWith("path:"),
    });
  }
}

function applyEvent(ev: any, focusFallback: string) {
  const s = getGraphSession();
  if (ev.incomplete != null) s.incomplete = !!ev.incomplete;

  switch (ev.type) {
    case "focus":
      if (ev.node) {
        applyNode(ev.node);
        s.focusId = ev.node.id;
      }
      break;
    case "node":
      if (ev.node) applyNode(ev.node);
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
      break;
    case "error":
      throw new Error(ev.message || "graph stream error");
  }
}

/**
 * Progressive graph expansion via SSE.
 * mode=project → project import map; otherwise neighborhood for ref.
 */
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
  let eventType = "message";
  const focusFallback = opts.ref || "path:./";

  const flushEvent = (type: string, data: string) => {
    if (!data) return;
    const ev = JSON.parse(data);
    // SSE event field overrides type if present
    if (type && type !== "message") ev.type = type;
    applyEvent(ev, focusFallback);
    handlers.onEvent?.();
    if (ev.type === "done") {
      handlers.onDone?.();
    }
  };

  try {
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buf += dec.decode(value, { stream: true });
      // Parse SSE blocks separated by blank lines
      for (;;) {
        const sep = buf.indexOf("\n\n");
        if (sep < 0) break;
        const block = buf.slice(0, sep);
        buf = buf.slice(sep + 2);
        let et = "message";
        const dataLines: string[] = [];
        for (const line of block.split("\n")) {
          if (line.startsWith("event:")) {
            et = line.slice(6).trim();
          } else if (line.startsWith("data:")) {
            dataLines.push(line.slice(5).trim());
          }
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
