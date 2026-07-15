export function assist() {
  return 1;
}
export function stay() {
  return 2;
}
export function use() {
  return `${assist()}-${stay()}`;
}
