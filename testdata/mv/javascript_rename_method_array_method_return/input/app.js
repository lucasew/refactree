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
  return [new BoxA().get()][0].run() + [new BoxB().get()][0].run();
}

function useArrayOf() {
  return Array.of(new BoxA().get())[0].run() + Array.of(new BoxB().get())[0].run();
}

function useAt() {
  return [new BoxA().get()].at(0).run() + [new BoxB().get()].at(0).run();
}

function usePop() {
  return [new BoxA().get()].pop().run() + [new BoxB().get()].pop().run();
}

function useValues() {
  return (
    Object.values({ k: new BoxA().get() })[0].run() +
    Object.values({ k: new BoxB().get() })[0].run()
  );
}

function useMapGet() {
  return (
    new Map([["k", new BoxA().get()]]).get("k").run() +
    new Map([["k", new BoxB().get()]]).get("k").run()
  );
}

function useSetSpread() {
  return (
    [...new Set([new BoxA().get()])][0].run() +
    [...new Set([new BoxB().get()])][0].run()
  );
}

function useSpread() {
  return [...[new BoxA().get()]][0].run() + [...[new BoxB().get()]][0].run();
}

function useNullish() {
  return (new BoxA().get() ?? null).run() + (new BoxB().get() ?? null).run();
}

function useOr() {
  return (new BoxA().get() || null).run() + (new BoxB().get() || null).run();
}

function useForOf() {
  let n = 0;
  for (const a of [new BoxA().get()]) {
    n += a.run();
  }
  for (const b of [new BoxB().get()]) {
    n += b.run();
  }
  return n;
}

function useLocal() {
  const ba = new BoxA();
  const bb = new BoxB();
  return [ba.get()][0].run() + [bb.get()][0].run();
}

function usePreservesB() {
  return (
    [new BoxB().get()][0].run() +
    Array.of(new BoxB().get())[0].run() +
    [new BoxB().get()].at(0).run() +
    [new BoxB().get()].pop().run() +
    Object.values({ k: new BoxB().get() })[0].run() +
    new Map([["k", new BoxB().get()]]).get("k").run() +
    (new BoxB().get() ?? null).run()
  );
}
