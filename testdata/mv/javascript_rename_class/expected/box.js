export class Crate {
  constructor(n) {
    this.n = n;
  }
  static make(n) {
    return new Crate(n);
  }
  helper() {
    return this.n;
  }
}
export class Sub extends Crate {
  constructor(n) {
    super(n);
  }
}
