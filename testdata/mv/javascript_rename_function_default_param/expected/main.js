import { assist, stay } from "./helper.js"

export function make(n = assist()) {
  return n + stay()
}

export function run() {
  return assist() + stay()
}
