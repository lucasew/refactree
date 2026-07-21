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
  } else if (symbol !== "" || s.includes("/") || s.startsWith(".")) {
    // Bare path / shorthand (Go Parse treats these as path provider).
    base = "path:./" + s.replace(/^\.\//, "");
  } else if (s === "" || s === ".") {
    base = "path:./";
  } else {
    // Bare identifier without provider — leave as-is (not a path ref).
    base = s;
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

/** Display-only: strip path:./ for module line; never use as an id. */
export function displayModulePath(refOrLabel: string): string {
  const s = (refOrLabel ?? "").trim();
  if (s.startsWith("path:")) {
    return s.slice("path:".length).replace(/^\.\//, "") || ".";
  }
  return s;
}
