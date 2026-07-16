export function helper() {
  return 1;
}
export function stay() {
  return 2;
}
export function use() {
  const o = { helper, stay };
  const { helper: h, stay: s } = o;
  return h() + s();
}
