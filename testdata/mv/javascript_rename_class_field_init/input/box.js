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
export class Holder {
  b = new Box(1)
  s = new Stay(2)
  getN() {
    return this.b.n
  }
}
