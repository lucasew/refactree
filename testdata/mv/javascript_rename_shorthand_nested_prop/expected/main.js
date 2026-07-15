export function assist() {
  return 1;
}
export function stay() {
  return 2;
}
export function use() {
  const o = { nested: { assist }, stay };
  return o.nested.assist() + o.stay();
}
