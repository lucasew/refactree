export function use() {
  return import("./box.js").then((m) => m.assist() + m.stay());
}

export function useArrow() {
  return import("./box.js").then(m => m.assist() + m.stay());
}
