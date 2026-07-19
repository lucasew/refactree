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

function useSpreadIndex() {
  const ma = new Map();
  const mb = new Map();
  ma.set("k", new A());
  mb.set("k", new B());
  return [...ma.values()][0].execute() + [...mb.values()][0].run();
}

function useSpreadIndexVar() {
  const ma = new Map();
  const mb = new Map();
  ma.set("k", new A());
  mb.set("k", new B());
  const a = [...ma.values()][0];
  const b = [...mb.values()][0];
  return a.execute() + b.run();
}

function useArrayFrom() {
  const ma = new Map();
  const mb = new Map();
  ma.set("k", new A());
  mb.set("k", new B());
  return Array.from(ma.values())[0].execute() + Array.from(mb.values())[0].run();
}

function useAt() {
  const ma = new Map();
  const mb = new Map();
  ma.set("k", new A());
  mb.set("k", new B());
  return [...ma.values()].at(0).execute() + [...mb.values()].at(0).run();
}

function useForEachValues() {
  const ma = new Map();
  const mb = new Map();
  ma.set("k", new A());
  mb.set("k", new B());
  let n = 0;
  ma.values().forEach((a) => {
    n += a.execute();
  });
  mb.values().forEach((b) => {
    n += b.run();
  });
  return n;
}

function useCtorSpread() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  return [...ma.values()][0].execute() + [...mb.values()][0].run();
}

function usePreservesB() {
  const mb = new Map();
  mb.set("k", new B());
  return [...mb.values()][0].run();
}
