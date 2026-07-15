export function use() {
  return import("./box.js").then(({ assist, stay }) => assist() + stay());
}
