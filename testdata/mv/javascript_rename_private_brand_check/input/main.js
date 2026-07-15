export class Box {
  #helper = 1;
  #stay = 2;
  use() {
    return #helper in this && this.#helper + this.#stay;
  }
}
export class Other {
  #helper = 9;
  check() {
    return #helper in this;
  }
}
