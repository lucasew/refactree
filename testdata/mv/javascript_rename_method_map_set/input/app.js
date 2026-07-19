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

function useSetGet() {
  const ma = new Map();
  const mb = new Map();
  ma.set("k", new A());
  mb.set("k", new B());
  return ma.get("k").run() + mb.get("k").run();
}

function useSetGetVar() {
  const ma = new Map();
  const mb = new Map();
  ma.set("k", new A());
  mb.set("k", new B());
  const a = ma.get("k");
  const b = mb.get("k");
  return a.run() + b.run();
}

function useSetValuesForOf() {
  const ma = new Map();
  const mb = new Map();
  ma.set("k", new A());
  mb.set("k", new B());
  let n = 0;
  for (const a of ma.values()) {
    n += a.run();
  }
  for (const b of mb.values()) {
    n += b.run();
  }
  return n;
}

function useWeakMapSet() {
  const ka = {};
  const kb = {};
  const ma = new WeakMap();
  const mb = new WeakMap();
  ma.set(ka, new A());
  mb.set(kb, new B());
  return ma.get(ka).run() + mb.get(kb).run();
}

function usePreservesB() {
  const mb = new Map();
  mb.set("k", new B());
  return mb.get("k").run();
}
