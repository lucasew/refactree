/**
 * Long-lived graph explore session (WebSocket).
 * Client visits refs; server pushes only edges not yet seen in the session.
 * Nodes (except focus) are stubbed from edges and hydrated via GraphQL nodes(ids).
 */

import {
  getGraphSession,
  isExternalId,
  linkKey,
  markExpanded,
  type IncomingEdge,
  type IncomingNode,
} from "./graphSession";
import { inferLanguageFromId } from "./graphColors";
import { formatGraphLabel, normalizeRef } from "./routes";

export type StreamHandlers = {
  onEvent?: () => void;
  onDone?: (visitRef?: string) => void;
  onError?: (err: Error) => void;
};

type ServerMsg = {
  type: string;
  node?: IncomingNode & { parentId?: string | null };
  edge?: { from: string; to: string; kind: string };
  incomplete?: boolean;
  message?: string;
  visitRef?: string;
};

const pendingNodeIds = new Set<string>();
let hydrateTimer: ReturnType<typeof setTimeout> | null = null;
let hydrateInFlight = false;

type Listener = (msg: ServerMsg) => void;

class GraphExploreClient {
  private ws: WebSocket | null = null;
  private ready = false;
  private readyWaiters: Array<() => void> = [];
  private listeners = new Set<Listener>();
  private queue: object[] = [];
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private closed = false;

  connect() {
    if (this.closed) return;
    if (
      this.ws &&
      (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING)
    ) {
      return;
    }
    const proto = window.location.protocol === "https:" ? "wss:" : "ws:";
    const url = `${proto}//${window.location.host}/api/graph/session`;
    const ws = new WebSocket(url);
    this.ws = ws;
    this.ready = false;

    ws.onmessage = (ev) => {
      let msg: ServerMsg;
      try {
        msg = JSON.parse(String(ev.data));
      } catch {
        return;
      }
      if (msg.type === "ready") {
        this.ready = true;
        const waiters = this.readyWaiters.splice(0);
        waiters.forEach((w) => w());
        for (const m of this.queue.splice(0)) {
          ws.send(JSON.stringify(m));
        }
      }
      this.applyServerMsg(msg);
      for (const l of this.listeners) l(msg);
    };
    ws.onclose = () => {
      this.ready = false;
      this.ws = null;
      if (!this.closed) {
        this.reconnectTimer = setTimeout(() => this.connect(), 800);
      }
    };
    ws.onerror = () => {
      /* onclose reconnects */
    };
  }

  private send(msg: object) {
    this.connect();
    if (this.ws && this.ready && this.ws.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg));
    } else {
      this.queue.push(msg);
    }
  }

  whenReady(): Promise<void> {
    this.connect();
    if (this.ready) return Promise.resolve();
    return new Promise((resolve) => {
      this.readyWaiters.push(resolve);
    });
  }

  subscribe(fn: Listener): () => void {
    this.listeners.add(fn);
    this.connect();
    return () => this.listeners.delete(fn);
  }

  visit(ref: string) {
    const id = normalizeRef(ref || "path:./");
    // Optimistic package/module node so file-browser navigation paints
    // immediately even while the server worker is mid-Materialize on another package.
    applyNode(stubFromId(id), true);
    getGraphSession().focusId = id;
    this.send({ op: "visit", ref: id });
  }

  project() {
    this.send({ op: "project" });
  }

  /** Sticky crawl flag: when true, worker auto-runs project crawl while free (after visits). */
  setCrawl(enabled: boolean) {
    this.send({ op: "crawl", enabled });
  }

  private applyServerMsg(msg: ServerMsg) {
    const s = getGraphSession();
    if (msg.incomplete != null) s.incomplete = !!msg.incomplete;

    switch (msg.type) {
      case "focus":
        if (msg.node) {
          applyNode(msg.node, true);
          s.focusId = normalizeRef(msg.node.id);
          if (msg.node.parentId) ensureStub(msg.node.parentId);
        }
        break;
      case "edge":
        if (msg.edge) {
          applyEdge({
            from: msg.edge.from,
            to: msg.edge.to,
            kind: msg.edge.kind,
          });
        }
        break;
      case "error":
        console.warn("graph session error:", msg.message);
        break;
      case "done":
        scheduleHydrate();
        break;
      default:
        break;
    }
  }
}

const client = new GraphExploreClient();

function stubFromId(rawId: string): IncomingNode {
  // Id + label keep path:./… so the canvas never shows bare cmd/rft.
  const id = normalizeRef(rawId);
  const external = isExternalId(id);
  let kind = "MODULE";
  if (id.includes("::")) kind = "ATOM";
  const label = formatGraphLabel(id, "reference");
  return { id, kind, label, external, expandable: external, language: inferLanguageFromId(id) };
}

function applyNode(n: IncomingNode, _markResolved = true) {
  const s = getGraphSession();
  const id = normalizeRef(n.id);
  // Always label from id so server short labels (cmd/rft) cannot drop path:./.
  const name = formatGraphLabel(id, "reference");
  const existing = s.nodes.get(id);
  if (existing) {
    existing.name = name;
    existing.kind = n.kind || existing.kind;
    if (n.external != null) existing.external = !!n.external;
    if (n.expandable != null) existing.expandable = !!n.expandable;
    if (n.language) existing.language = n.language;
  } else {
    s.nodes.set(id, {
      id,
      name,
      kind: n.kind,
      external: !!n.external,
      expandable: !!n.expandable,
      language: n.language || inferLanguageFromId(id),
    });
  }
}

function ensureStub(rawId: string) {
  if (!rawId) return;
  const id = normalizeRef(rawId);
  if (!getGraphSession().nodes.has(id)) {
    applyNode(stubFromId(id), false);
  }
  pendingNodeIds.add(id);
  scheduleHydrate();
}

function applyEdge(e: IncomingEdge) {
  if (!e.from || !e.to) return;
  const from = normalizeRef(e.from);
  const to = normalizeRef(e.to);
  const s = getGraphSession();
  const k = linkKey(from, to, e.kind);
  if (!s.links.has(k)) {
    s.links.set(k, { source: from, target: to, kind: e.kind });
  }
  ensureStub(from);
  ensureStub(to);
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
          nodes(ids: $ids) { id kind label parentId external expandable language }
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
    for (const id of ids) pendingNodeIds.add(id);
  } finally {
    hydrateInFlight = false;
    if (pendingNodeIds.size > 0) scheduleHydrate();
  }
}

function runOp(
  start: () => void,
  isDone: (msg: ServerMsg) => boolean,
  handlers: StreamHandlers,
  timeoutMs: number
): Promise<void> {
  return client.whenReady().then(
    () =>
      new Promise((resolve) => {
        const unsub = client.subscribe((msg) => {
          handlers.onEvent?.();
          if (msg.type === "error") {
            handlers.onError?.(new Error(msg.message || "graph session error"));
          }
          if (isDone(msg)) {
            unsub();
            handlers.onDone?.(msg.visitRef);
            resolve();
          }
        });
        start();
        setTimeout(() => {
          unsub();
          resolve();
        }, timeoutMs);
      })
  );
}

/** Visit a ref in the shared session; only new edges are pushed. */
export function sessionVisit(ref: string, handlers: StreamHandlers = {}): Promise<void> {
  const want = normalizeRef(ref || "path:./");
  return runOp(
    () => client.visit(want),
    (msg) =>
      msg.type === "done" &&
      !!msg.visitRef &&
      msg.visitRef !== "project" &&
      normalizeRef(msg.visitRef) === want,
    handlers,
    120_000
  );
}

/** Grow project import map (session deltas). */
export function sessionProject(handlers: StreamHandlers = {}): Promise<void> {
  return runOp(
    () => client.project(),
    (msg) => msg.type === "done" && msg.visitRef === "project",
    handlers,
    300_000
  );
}

/**
 * Enable/disable sticky repo crawl on the server worker.
 * When enabled, the worker runs StreamProject when free (after visits), using
 * the ingest skip list. Disabling preempts an in-flight project crawl.
 */
export function setSessionCrawl(
  enabled: boolean,
  handlers: StreamHandlers = {}
): Promise<void> {
  if (!enabled) {
    client.setCrawl(false);
    return Promise.resolve();
  }
  return runOp(
    () => client.setCrawl(true),
    (msg) => msg.type === "done" && msg.visitRef === "project",
    handlers,
    300_000
  );
}

export async function streamExpandExternal(
  ref: string,
  handlers: StreamHandlers = {}
): Promise<void> {
  const id = normalizeRef(ref);
  await sessionVisit(id, handlers);
  markExpanded(id);
}

export function ensureGraphSession() {
  client.connect();
}

/** Shared WS client (for subscribing to auto-crawl edges while crawl is on). */
export function getGraphSessionClient() {
  return client;
}
