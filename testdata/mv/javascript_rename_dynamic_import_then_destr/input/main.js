export function use() {
  return import("./box.js").then(({ helper, stay }) => helper() + stay());
}
