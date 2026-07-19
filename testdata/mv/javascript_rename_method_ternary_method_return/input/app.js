class A {
  helper() { return 1; }
}
class B {
  helper() { return 2; }
}

class BoxA {
  #a = new A();
  get() { return this.#a; }
  self() { return this; }
}
class BoxB {
  #b = new B();
  get() { return this.#b; }
  self() { return this; }
}

export function useTernary(c) {
  return (c ? new BoxA().get() : new BoxA().get()).helper()
    + (c ? new BoxB().get() : new BoxB().get()).helper();
}
export function useTernaryRecv(c) {
  const ba = new BoxA();
  const bb = new BoxB();
  return (c ? ba : ba).get().helper() + (c ? bb : bb).get().helper();
}
export function useParenChain() {
  return (new BoxA().self()).get().helper() + (new BoxB().self()).get().helper();
}
export function usePreservesB(c) {
  const bb = new BoxB();
  return (c ? new BoxB().get() : new BoxB().get()).helper()
    + (c ? bb : bb).get().helper()
    + (new BoxB().self()).get().helper();
}
