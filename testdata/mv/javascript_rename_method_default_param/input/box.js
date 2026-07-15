export function helper() {
  return 1
}
export function stay() {
  return 2
}
export class Runner {
  run(n = helper()) {
    return n + stay()
  }
}
