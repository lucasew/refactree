export function use() {
  return import("./box.js").then((m) => m.helper() + m.stay());
}

export function useArrow() {
  return import("./box.js").then(m => m.helper() + m.stay());
}
