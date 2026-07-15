export function assist() {
  return 1
}
export function stay() {
  return 2
}
export function make(n = assist()) {
  return n + stay()
}
export const arrow = (n = assist()) => n + stay()
