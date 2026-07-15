export function assist() {
  return 1;
}
export function stay() {
  return 2;
}
export function use(c, f) {
  const a = (assist)();
  const b = (c ? assist : stay)();
  const d = (f || assist)();
  const e = (0, assist)();
  return a + b + d + e + stay();
}
