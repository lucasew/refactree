export function helper() {
  return 1
}
export function stay() {
  return 2
}
export function makeObj({ n = helper() } = {}) {
  return n + stay()
}
export function makeArr([n = helper()] = []) {
  return n + stay()
}
