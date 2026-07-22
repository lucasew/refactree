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
  let atom = "";
  const symIdx = s.indexOf("::");
  if (symIdx >= 0) {
    atom = s.slice(symIdx + 2);
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

  return symbol !== "" ? base + "::" + atom : base;
}

/** Parse browser path into a focus reference (canonical id). Hash is ignored. */
export function refFromPath(pathname: string): string {
  // Allow callers to pass pathname+hash or full path.
  const pathOnly = (pathname ?? "").split("#")[0] ?? "";
  if (pathOnly === "/" || pathOnly === "" || pathOnly === "/graph") {
    return "path:./";
  }
  const prefix = "/code/";
  if (!pathOnly.startsWith(prefix)) {
    return "path:./";
  }
  let rest = pathOnly.slice(prefix.length);
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

/**
 * Safe HTML element id for a symbol ref — mirrors pkg/web/annotate.AnchorID.
 * Used as URL hash so the code pane can scroll to the definition.
 */
export function anchorIdFromRef(ref: string): string {
  const id = normalizeRef(ref ?? "");
  if (!id.includes("::")) return "";
  let out = "sym-";
  for (const ch of id) {
    if (
      (ch >= "a" && ch <= "z") ||
      (ch >= "A" && ch <= "Z") ||
      (ch >= "0" && ch <= "9") ||
      ch === "-" ||
      ch === "_"
    ) {
      out += ch;
    } else {
      out += "_";
    }
  }
  return out;
}

/** Hash fragment for a ref (empty when no symbol). */
export function hashFromRef(ref: string): string {
  const a = anchorIdFromRef(ref);
  return a ? "#" + a : "";
}

export function pathFromRef(ref: string): string {
  const id = normalizeRef(ref);
  if (id === "path:./" || id === "path:.") {
    return "/";
  }
  // Include #sym-… for symbol refs (restored after React migration).
  return "/code/" + encodeURIComponent(id) + hashFromRef(id);
}

export function navigateToRef(ref: string) {
  const path = pathFromRef(ref);
  const cur = window.location.pathname + window.location.hash;
  if (cur !== path) {
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
export function refDisplayParts(id: string): { module: string; atom: string } {
  const raw = normalizeRef(id ?? "");
  let base = raw;
  let atom = "";
  const symIdx = raw.indexOf("::");
  if (symIdx >= 0) {
    atom = raw.slice(symIdx + 2);
    base = raw.slice(0, symIdx);
  }
  return { module: base || "path:./", atom };
}

/**
 * Module scope id: strip ::symbol so path:./cmd/rft::Main → path:./cmd/rft.
 * Used by module graph view (one node per module).
 */
export function moduleScopeId(ref: string): string {
  const id = normalizeRef(ref);
  const i = id.indexOf("::");
  return i >= 0 ? id.slice(0, i) : id;
}

/**
 * Extensions whose languages use directory modules (go, java) — match
 * ingest.LanguageUsesDirectoryModule. File path:./pkg/web/server.go → package path:./pkg/web.
 */
const DIRECTORY_MODULE_EXT = new Set([
  ".go",
  ".java",
]);

/**
 * Graph node id: same idea as backend graphRefID / projectScopeID.
 * Collapses file paths for directory-module languages so the canvas shows
 * modules (path:./pkg/web) not files (path:./pkg/web/server.go).
 */
export function graphScopeId(ref: string): string {
  const id = normalizeRef(ref);
  let base = id;
  let atom = "";
  const symIdx = id.indexOf("::");
  if (symIdx >= 0) {
    atom = id.slice(symIdx + 2);
    base = id.slice(0, symIdx);
  }

  const colon = base.indexOf(":");
  if (colon <= 0) {
    return atom ? base + "::" + atom : base;
  }
  const prov = base.slice(0, colon).toLowerCase();
  let path = base.slice(colon + 1);

  if (prov === "path") {
    path = path.replace(/^\.\//, "");
    if (path === "" || path === ".") {
      base = "path:./";
    } else {
      const slash = path.lastIndexOf("/");
      const file = slash >= 0 ? path.slice(slash + 1) : path;
      const dot = file.lastIndexOf(".");
      const ext = dot >= 0 ? file.slice(dot).toLowerCase() : "";
      if (ext && DIRECTORY_MODULE_EXT.has(ext)) {
        const dir = slash >= 0 ? path.slice(0, slash) : "";
        base = dir ? "path:./" + dir : "path:./";
      } else {
        base = "path:./" + path;
      }
    }
  }
  // external go:fmt / go:fmt::Println — keep provider path as module
  return atom ? base + "::" + atom : base;
}

export type GraphViewMode = "module" | "atom";
/**
 * Canvas / tooltip label for a node — always shows canonical path:./…
 * module: one line (path:./cmd/rft)
 * atom: two lines when an atom name exists (path:./cmd/rft\\nname)
 */
export function formatGraphLabel(id: string, mode: GraphViewMode): string {
  const { module, atom } = refDisplayParts(id);
  if (mode === "module" || !atom) return module;
  return `${module}\n${atom}`;
}
