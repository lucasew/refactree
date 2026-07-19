class A {
  renamed() { return 1; }
  static create() { return new A(); }
}
class B {
  helper() { return 2; }
  static create() { return new B(); }
}

class BoxA {
  #a = new A();
  a = new A();
  get() { return this.#a; }
  #get() { return this.#a; }
  usePrivate() { return this.#get().renamed(); }
}
class BoxB {
  #b = new B();
  b = new B();
  get() { return this.#b; }
  #get() { return this.#b; }
  usePrivate() { return this.#get().helper(); }
}

class OuterA {
  box = new BoxA();
}
class OuterB {
  box = new BoxB();
}

export function useGet() {
  return new BoxA().get().renamed() + new BoxB().get().helper();
}
export function useGetAssign() {
  const a = new BoxA().get();
  const b = new BoxB().get();
  return a.renamed() + b.helper();
}
export function useStatic() {
  return A.create().renamed() + B.create().helper();
}
export function useStaticAssign() {
  const a = A.create();
  const b = B.create();
  return a.renamed() + b.helper();
}
export function useNested() {
  return new OuterA().box.a.renamed() + new OuterB().box.b.helper();
}
export function useNestedAssign() {
  const oa = new OuterA();
  const ob = new OuterB();
  return oa.box.a.renamed() + ob.box.b.helper();
}
export function usePrivate() {
  return new BoxA().usePrivate() + new BoxB().usePrivate();
}
export function usePreservesB() {
  return new BoxB().get().helper() + B.create().helper() + new OuterB().box.b.helper();
}
