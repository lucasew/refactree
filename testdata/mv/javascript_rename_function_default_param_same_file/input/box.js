export function helper() {
  return 1
}
export function stay() {
  return 2
}
export function make(n = helper()) {
  return n + stay()
}
export const arrow = (n = helper()) => n + stay()
