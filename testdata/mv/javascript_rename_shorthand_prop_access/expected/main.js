export function assist() {
  return 1;
}
export function stay() {
  return 2;
}
export function use() {
  const o = { assist, stay };
  return o.assist() + o.stay();
}
