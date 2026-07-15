export class Box {
  constructor(n) {
    this.n = n
  }
}
export class Stay {
  constructor(n) {
    this.n = n
  }
}
export function make(b = new Box(0)) {
  return b.n
}
export function other(s = new Stay(0)) {
  return s.n
}
