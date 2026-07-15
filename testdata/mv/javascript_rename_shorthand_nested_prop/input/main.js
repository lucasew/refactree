export function helper() {
  return 1;
}
export function stay() {
  return 2;
}
export function use() {
  const o = { nested: { helper }, stay };
  return o.nested.helper() + o.stay();
}
