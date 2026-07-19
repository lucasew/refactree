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

function useSymbolIteratorNext() {
  const ma = new Map();
  const mb = new Map();
  ma.set("k", new A());
  mb.set("k", new B());
  return (
    ma[Symbol.iterator]().next().value[1].run() +
    mb[Symbol.iterator]().next().value[1].run()
  );
}

function useSymbolIteratorCtor() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  return (
    ma[Symbol.iterator]().next().value[1].run() +
    mb[Symbol.iterator]().next().value[1].run()
  );
}

function useSymbolIteratorInline() {
  return (
    new Map([["k", new A()]])[Symbol.iterator]().next().value[1].run() +
    new Map([["k", new B()]])[Symbol.iterator]().next().value[1].run()
  );
}

function useSymbolIteratorLocal() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  const ia = ma[Symbol.iterator]();
  const ib = mb[Symbol.iterator]();
  return ia.next().value[1].run() + ib.next().value[1].run();
}

function useBareSpreadAt() {
  const ma = new Map();
  const mb = new Map();
  ma.set("k", new A());
  mb.set("k", new B());
  return [...ma].at(0)[1].run() + [...mb].at(0)[1].run();
}

function useBareArrayFromAt() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  return Array.from(ma).at(0)[1].run() + Array.from(mb).at(0)[1].run();
}

function useEntriesSpreadAt() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  return (
    [...ma.entries()].at(0)[1].run() + [...mb.entries()].at(0)[1].run()
  );
}

function useBareSpreadAtVar() {
  const ma = new Map();
  const mb = new Map();
  ma.set("k", new A());
  mb.set("k", new B());
  const a = [...ma].at(0)[1];
  const b = [...mb].at(0)[1];
  return a.run() + b.run();
}

function useBareSpreadAtPairVar() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  const ea = [...ma].at(0);
  const eb = [...mb].at(0);
  return ea[1].run() + eb[1].run();
}

function usePreservesB() {
  const mb = new Map();
  mb.set("k", new B());
  return (
    mb[Symbol.iterator]().next().value[1].run() + [...mb].at(0)[1].run()
  );
}
