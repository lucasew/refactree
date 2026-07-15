export class Base {
  helper() { return 1 }
  stay() { return 2 }
}

export class Box extends Base {
  assist() { return super.helper() + 10 }
  use() { return this.assist() + this.stay() }
}
