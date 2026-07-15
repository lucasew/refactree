export function helper() {
  return 1;
}
export function stay() {
  return 2;
}
export function use(c, f) {
  const a = (helper)();
  const b = (c ? helper : stay)();
  const d = (f || helper)();
  const e = (0, helper)();
  return a + b + d + e + stay();
}
