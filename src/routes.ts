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
  return rest;
}

export function pathFromRef(ref: string): string {
  if (!ref || ref === "path:./" || ref === "path:.") {
    return "/";
  }
  return "/code/" + encodeURIComponent(ref);
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
