/**
 * Graph node colors: fill = language, stroke = internal (path) vs external (provider).
 * Canvas paints stay hex (theme-independent) per PRODUCT chart guidance.
 */

/** Language → fill (dark-canvas friendly). */
const LANGUAGE_FILL: Record<string, string> = {
  go: "#00ADD8",
  python: "#4B8BBE",
  javascript: "#C4A035",
  typescript: "#3178C6",
  java: "#E76F00",
  svelte: "#FF3E00",
  nix: "#7EBAE4",
};

const UNKNOWN_FILL = "#6B7280";
const FOCUS_RING = "#F5D76E";
const EXTERNAL_RING = "#E8A838";
const INTERNAL_RING = "rgba(232,224,208,0.35)";

export function languageFill(language: string | undefined | null): string {
  if (!language) return UNKNOWN_FILL;
  return LANGUAGE_FILL[language.toLowerCase()] ?? UNKNOWN_FILL;
}

/** External nodes: slightly dimmer fill so language still reads. */
export function nodeFill(opts: {
  language?: string | null;
  external?: boolean;
  focused?: boolean;
}): string {
  const base = languageFill(opts.language);
  if (opts.focused) {
    // keep language hue; focus is stroke
    return base;
  }
  if (opts.external) {
    return dimHex(base, 0.72);
  }
  return base;
}

export function nodeStroke(opts: {
  external?: boolean;
  focused?: boolean;
  expandable?: boolean;
}): { color: string; width: number } | null {
  if (opts.focused) {
    return { color: FOCUS_RING, width: 2.5 };
  }
  if (opts.external) {
    // Always mark external; thicker if still expandable
    return {
      color: EXTERNAL_RING,
      width: opts.expandable !== false ? 2 : 1.25,
    };
  }
  // Subtle ring for internal path: code
  return { color: INTERNAL_RING, width: 1 };
}

/** Crude darken/mix toward background for external. */
function dimHex(hex: string, amount: number): string {
  const h = hex.replace("#", "");
  if (h.length !== 6) return hex;
  const r = parseInt(h.slice(0, 2), 16);
  const g = parseInt(h.slice(2, 4), 16);
  const b = parseInt(h.slice(4, 6), 16);
  const bg = 0x1a; // ~#1a1814
  const mix = (c: number) => Math.round(c * amount + bg * (1 - amount));
  const to = (c: number) => c.toString(16).padStart(2, "0");
  return `#${to(mix(r))}${to(mix(g))}${to(mix(b))}`;
}

/** Infer language client-side when hydrate has not landed yet. */
export function inferLanguageFromId(id: string): string {
  if (!id) return "";
  const colon = id.indexOf(":");
  if (colon > 0) {
    const prov = id.slice(0, colon).toLowerCase();
    if (prov === "go") return "go";
    if (prov === "python") return "python";
    if (prov === "node" || prov === "javascript" || prov === "js") return "javascript";
    if (prov === "java") return "java";
    if (prov === "nix") return "nix";
    if (prov === "svelte") return "svelte";
    if (prov === "path") {
      const path = id.slice(colon + 1).replace(/^\.\//, "").split("::")[0];
      const ext = path.includes(".") ? path.slice(path.lastIndexOf(".")).toLowerCase() : "";
      const byExt: Record<string, string> = {
        ".go": "go",
        ".py": "python",
        ".js": "javascript",
        ".jsx": "javascript",
        ".mjs": "javascript",
        ".cjs": "javascript",
        ".ts": "typescript",
        ".tsx": "typescript",
        ".java": "java",
        ".svelte": "svelte",
        ".nix": "nix",
      };
      return byExt[ext] ?? "";
    }
  }
  return "";
}
