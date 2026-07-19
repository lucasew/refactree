class A {
  renamed() { return 1; }
}
class B {
  helper() { return 2; }
}

class BoxA {
  #a = new A();
  a = new A();
  static #sa = new A();
  use() {
    return this.#a.renamed() + this.a.renamed();
  }
  static useStatic() {
    return this.#sa.renamed() + BoxA.#sa.renamed();
  }
}
class BoxB {
  #b = new B();
  b = new B();
  static #sb = new B();
  use() {
    return this.#b.helper() + this.b.helper();
  }
  static useStatic() {
    return this.#sb.helper() + BoxB.#sb.helper();
  }
}

export function use() {
  return new BoxA().use() + new BoxB().use() + BoxA.useStatic() + BoxB.useStatic();
}
