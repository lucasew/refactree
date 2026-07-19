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

function useNullishGet() {
  const ma = new Map();
  const mb = new Map();
  ma.set("k", new A());
  mb.set("k", new B());
  return (ma.get("k") ?? new A()).run() + (mb.get("k") ?? new B()).run();
}

function useOrGet() {
  const ma = new Map();
  const mb = new Map();
  ma.set("k", new A());
  mb.set("k", new B());
  return (ma.get("k") || new A()).run() + (mb.get("k") || new B()).run();
}

function useNullishGetVar() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  const a = ma.get("k") ?? new A();
  const b = mb.get("k") ?? new B();
  return a.run() + b.run();
}

function useOrGetVar() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  const a = ma.get("k") || new A();
  const b = mb.get("k") || new B();
  return a.run() + b.run();
}

function useNullishNullDefault() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  return (ma.get("k") ?? null).run() + (mb.get("k") ?? null).run();
}

function useNullishUndefinedDefault() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  return (ma.get("k") ?? undefined).run() + (mb.get("k") ?? undefined).run();
}

function useWeakMapNullish() {
  const ka = {};
  const kb = {};
  const ma = new WeakMap();
  const mb = new WeakMap();
  ma.set(ka, new A());
  mb.set(kb, new B());
  return (ma.get(ka) ?? new A()).run() + (mb.get(kb) ?? new B()).run();
}

function usePreservesB() {
  const mb = new Map();
  mb.set("k", new B());
  return (mb.get("k") ?? new B()).run() + (mb.get("k") || new B()).run();
}
