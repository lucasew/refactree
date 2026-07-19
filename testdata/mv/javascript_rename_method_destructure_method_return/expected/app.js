class A {
  execute() {
    return 1;
  }
}

class B {
  run() {
    return 2;
  }
}

class BoxA {
  a = new A();
  get() {
    return this.a;
  }
}

class BoxB {
  b = new B();
  get() {
    return this.b;
  }
}

function useArrayDestructure() {
  const [arrA] = [new BoxA().get()];
  const [arrB] = [new BoxB().get()];
  return arrA.execute() + arrB.run();
}

function useObjectDestructure() {
  const { k: objA } = { k: new BoxA().get() };
  const { k: objB } = { k: new BoxB().get() };
  return objA.execute() + objB.run();
}

function useLocal() {
  const ba = new BoxA();
  const bb = new BoxB();
  const [locA] = [ba.get()];
  const { k: locB } = { k: bb.get() };
  return locA.execute() + locB.run();
}

function usePreservesB() {
  const [arrB] = [new BoxB().get()];
  const { k: objB } = { k: new BoxB().get() };
  return arrB.run() + objB.run();
}
