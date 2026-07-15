export function assist() {
  return 1;
}
export function stay() {
  return 2;
}
export function use() {
  const o = { helper: assist, stay: stay };
  return o.helper() + o.stay();
}
