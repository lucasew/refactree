export class Base {
  assist() { return 1 }
  stay() { return 2 }
}

export class Box extends Base {
  helper() { return super.assist() + 10 }
  use() { return this.helper() + this.stay() }
}
