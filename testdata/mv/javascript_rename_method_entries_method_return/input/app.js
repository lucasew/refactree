class A {
  run() {
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

function useIndex() {
  return (
    Object.entries({ k: new BoxA().get() })[0][1].run() +
    Object.entries({ k: new BoxB().get() })[0][1].run()
  );
}

function usePairDestructure() {
  const [, xa] = Object.entries({ k: new BoxA().get() })[0];
  const [, xb] = Object.entries({ k: new BoxB().get() })[0];
  return xa.run() + xb.run();
}

function useForOf() {
  let n = 0;
  for (const [, xa] of Object.entries({ k: new BoxA().get() })) {
    n += xa.run();
  }
  for (const [, xb] of Object.entries({ k: new BoxB().get() })) {
    n += xb.run();
  }
  return n;
}

function useAssign() {
  const xa = Object.entries({ k: new BoxA().get() })[0][1];
  const xb = Object.entries({ k: new BoxB().get() })[0][1];
  return xa.run() + xb.run();
}

function useLocal() {
  const ba = new BoxA();
  const bb = new BoxB();
  return (
    Object.entries({ k: ba.get() })[0][1].run() +
    Object.entries({ k: bb.get() })[0][1].run()
  );
}

function usePreservesB() {
  const [, xb] = Object.entries({ k: new BoxB().get() })[0];
  return xb.run() + Object.entries({ k: new BoxB().get() })[0][1].run();
}
