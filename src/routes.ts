/**
 * Canonicalize a reference string for navigation / graph ids.
 * Matches pkg/reference.Parse + path normalization:
 *   bare "cmd/rft" → "path:./cmd/rft"
 *   "cmd/rft::main" → "path:./cmd/rft::main"
 * Display labels may omit the prefix; ids must not.
 */
export function normalizeRef(ref: string): string {
  let s = (ref ?? "").trim();
  if (!s) return "path:./";

  // Symbol first (same as Go Parse): provider:path::symbol
  let symbol = "";
  const symIdx = s.indexOf("::");
  if (symIdx >= 0) {
    symbol = s.slice(symIdx + 2);
    s = s.slice(0, symIdx);
  }

  let base: string;
  // Provider colon — not confused with :: (already stripped).
  const colon = s.indexOf(":");
  if (colon > 0 && !s.startsWith("./") && !s.startsWith("../") && !s.startsWith("/")) {
    const prov = s.slice(0, colon).toLowerCase();
    let path = s.slice(colon + 1);
    if (prov === "path") {
      if (path === "" || path === ".") path = "./";
      else if (!path.startsWith("./") && !path.startsWith("../") && !path.startsWith("/")) {
        path = "./" + path;
      }
      base = "path:" + path;
    } else {
      base = prov + ":" + path;
    }
  } else if (s.startsWith("./") || s.startsWith("../") || s.startsWith("/")) {
    base = "path:" + s;
  } else if (s === "" || s === ".") {
    base = "path:./";
  } else {
    // Bare token (cmd, cmd/rft, main.go, …) → project path ref.
    // External providers always arrive as "go:…", "node:…", already handled above.
    // Folder clicks and /code/cmd URLs must not stay as bare "cmd".
    base = "path:./" + s.replace(/^\.\//, "");
  }

  return symbol !== "" ? base + "::" + symbol : base;
}

/** Parse browser path into a focus reference (canonical id). */
export function refFromPath(pathname: string): string {
  if (pathname === "/" || pathname === "" || pathname === "/graph") {
    return "path:./";
  }
  const prefix = "/code/";
  if (!pathname.startsWith(prefix)) {
    return "path:./";
  }
  let rest = pathname.slice(prefix.length);
  try {
    rest = decodeURIComponent(rest);
  } catch {
    /* keep raw */
  }
  rest = rest.replace(/\/+$/, "");
  if (!rest) {
    return "path:./";
  }
  return normalizeRef(rest);
}

export function pathFromRef(ref: string): string {
  const id = normalizeRef(ref);
  if (id === "path:./" || id === "path:.") {
    return "/";
  }
  return "/code/" + encodeURIComponent(id);
}

export function navigateToRef(ref: string) {
  const path = pathFromRef(ref);
  if (window.location.pathname + window.location.search !== path) {
    window.history.pushState({}, "", path);
  }
  window.dispatchEvent(new PopStateEvent("popstate"));
}

export function navigateToGraph() {
  if (window.location.pathname !== "/graph") {
    window.history.pushState({}, "", "/graph");
  }
  window.dispatchEvent(new PopStateEvent("popstate"));
}

export function isGraphRoute(pathname: string): boolean {
  return pathname === "/graph" || pathname.startsWith("/graph/");
}

/**
 * Short path for dense UI (file rail names, etc.). Graph canvas keeps path:./.
 * Never use as an id.
 */
export function displayModulePath(refOrLabel: string): string {
  const s = (refOrLabel ?? "").trim();
  if (s.startsWith("path:")) {
    return s.slice("path:".length).replace(/^\.\//, "") || ".";
  }
  return s;
}

/** Split a ref id into module + optional symbol. Module line keeps path:./. */
export function refDisplayParts(id: string): { module: string; symbol: string } {
  const raw = normalizeRef(id ?? "");
  let base = raw;
  let symbol = "";
  const symIdx = raw.indexOf("::");
  if (symIdx >= 0) {
    symbol = raw.slice(symIdx + 2);
    base = raw.slice(0, symIdx);
  }
  return { module: base || "path:./", symbol };
}

/**
 * Package/module scope id: strip ::symbol so path:./cmd/rft::Main → path:./cmd/rft.
 * Used by package graph view (one node per package).
 */
export function packageScopeId(ref: string): string {
  const id = normalizeRef(ref);
  const i = id.indexOf("::");
  return i >= 0 ? id.slice(0, i) : id;
}

export type GraphViewMode = "package" | "reference";
/** @deprecated use GraphViewMode */
export type GraphLabelMode = GraphViewMode;

/**
 * Canvas / tooltip label for a node — always shows canonical path:./…
 * package: one line (path:./cmd/rft)
 * reference: two lines when a symbol exists (path:./cmd/rft\\nname)
 */
export function formatGraphLabel(id: string, mode: GraphViewMode): string {
  const { module, symbol } = refDisplayParts(id);
  if (mode === "package" || !symbol) return module;
  return `${module}\n${symbol}`;
}
