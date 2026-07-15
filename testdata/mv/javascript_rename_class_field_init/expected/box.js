export class Crate {
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
  b = new Crate(1)
  s = new Stay(2)
  getN() {
    return this.b.n
  }
}
