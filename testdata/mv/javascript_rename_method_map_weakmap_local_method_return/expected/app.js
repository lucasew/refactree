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

// Map/WeakMap local + set method-return under foreign same-leaf.
function useMapLocal() {
  const ma = new Map([["k", new BoxA().get()]]);
  const mb = new Map([["k", new BoxB().get()]]);
  return ma.get("k").execute() + mb.get("k").run();
}

function useMapSet() {
  const ma = new Map();
  ma.set("k", new BoxA().get());
  const mb = new Map();
  mb.set("k", new BoxB().get());
  return ma.get("k").execute() + mb.get("k").run();
}

function useWeakMapLocal() {
  const ka = {};
  const kb = {};
  const ma = new WeakMap([[ka, new BoxA().get()]]);
  const mb = new WeakMap([[kb, new BoxB().get()]]);
  return ma.get(ka).execute() + mb.get(kb).run();
}

function useWeakMapSet() {
  const ka = {};
  const kb = {};
  const ma = new WeakMap();
  ma.set(ka, new BoxA().get());
  const mb = new WeakMap();
  mb.set(kb, new BoxB().get());
  return ma.get(ka).execute() + mb.get(kb).run();
}

// Class regression — already worked.
function useClass() {
  const ka = {};
  const kb = {};
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  const wa = new WeakMap([[ka, new A()]]);
  const wb = new WeakMap([[kb, new B()]]);
  const sa = new Map();
  sa.set("k", new A());
  const sb = new Map();
  sb.set("k", new B());
  return (
    ma.get("k").execute() +
    mb.get("k").run() +
    wa.get(ka).execute() +
    wb.get(kb).run() +
    sa.get("k").execute() +
    sb.get("k").run()
  );
}

function usePreservesB() {
  const k = {};
  const mb = new Map([["k", new BoxB().get()]]);
  const wb = new WeakMap([[k, new BoxB().get()]]);
  const sb = new Map();
  sb.set("k", new BoxB().get());
  return (
    mb.get("k").run() +
    wb.get(k).run() +
    sb.get("k").run() +
    new Map([["k", new B()]]).get("k").run()
  );
}
