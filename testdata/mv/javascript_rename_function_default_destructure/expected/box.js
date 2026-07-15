export function assist() {
  return 1
}
export function stay() {
  return 2
}
export function makeObj({ n = assist() } = {}) {
  return n + stay()
}
export function makeArr([n = assist()] = []) {
  return n + stay()
}
