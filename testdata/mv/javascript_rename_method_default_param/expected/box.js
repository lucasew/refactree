export function assist() {
  return 1
}
export function stay() {
  return 2
}
export class Runner {
  run(n = assist()) {
    return n + stay()
  }
}
