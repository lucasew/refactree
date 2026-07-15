export class Box {
  constructor(n) {
    this.n = n;
  }
  static make(n) {
    return new Box(n);
  }
  helper() {
    return this.n;
  }
}
export class Sub extends Box {
  constructor(n) {
    super(n);
  }
}
