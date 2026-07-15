import { helper, stay } from "./helper.js"

export function make(n = helper()) {
  return n + stay()
}

export function run() {
  return helper() + stay()
}
