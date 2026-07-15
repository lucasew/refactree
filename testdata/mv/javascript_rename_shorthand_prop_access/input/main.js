export function helper() {
  return 1;
}
export function stay() {
  return 2;
}
export function use() {
  const o = { helper, stay };
  return o.helper() + o.stay();
}
