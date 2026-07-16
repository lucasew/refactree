export function helper() {
  return 1;
}
export function stay() {
  return 2;
}
export function use() {
  const o = { helper: helper, stay: stay };
  return o.helper() + o.stay();
}
