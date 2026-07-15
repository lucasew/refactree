export class Box {
  #assist = 1;
  #stay = 2;
  use() {
    return #assist in this && this.#assist + this.#stay;
  }
}
export class Other {
  #helper = 9;
  check() {
    return #helper in this;
  }
}
