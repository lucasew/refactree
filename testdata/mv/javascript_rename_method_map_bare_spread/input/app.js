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

function useBareSpreadSet() {
  const ma = new Map();
  const mb = new Map();
  ma.set("k", new A());
  mb.set("k", new B());
  return [...ma][0][1].run() + [...mb][0][1].run();
}

function useBareSpreadCtor() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  return [...ma][0][1].run() + [...mb][0][1].run();
}

function useBareArrayFrom() {
  const ma = new Map();
  const mb = new Map();
  ma.set("k", new A());
  mb.set("k", new B());
  return Array.from(ma)[0][1].run() + Array.from(mb)[0][1].run();
}

function useBareSpreadVar() {
  const ma = new Map();
  const mb = new Map();
  ma.set("k", new A());
  mb.set("k", new B());
  const a = [...ma][0][1];
  const b = [...mb][0][1];
  return a.run() + b.run();
}

function useBareArrayFromVar() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  const a = Array.from(ma)[0][1];
  const b = Array.from(mb)[0][1];
  return a.run() + b.run();
}

function useEntriesSpreadSet() {
  const ma = new Map();
  const mb = new Map();
  ma.set("k", new A());
  mb.set("k", new B());
  return [...ma.entries()][0][1].run() + [...mb.entries()][0][1].run();
}

function usePreservesB() {
  const mb = new Map();
  mb.set("k", new B());
  return [...mb][0][1].run() + Array.from(mb)[0][1].run();
}
