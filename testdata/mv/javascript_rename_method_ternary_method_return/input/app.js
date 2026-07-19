class A {
  helper() { return 1; }
}
class B {
  helper() { return 2; }
}

class BoxA {
  #a = new A();
  get() { return this.#a; }
}
class BoxB {
  #b = new B();
  get() { return this.#b; }
}

export function useTernary(c) {
  return (c ? new BoxA().get() : new BoxA().get()).helper()
    + (c ? new BoxB().get() : new BoxB().get()).helper();
}
export function usePreservesB(c) {
  return (c ? new BoxB().get() : new BoxB().get()).helper();
}
