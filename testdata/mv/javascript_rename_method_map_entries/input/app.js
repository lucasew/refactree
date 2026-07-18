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

function useMapEntriesForOfDestructure() {
  let n = 0;
  for (const [, xa] of new Map([["k", new A()]]).entries()) {
    n += xa.run();
  }
  for (const [, xb] of new Map([["k", new B()]]).entries()) {
    n += xb.run();
  }
  return n;
}

function useMapEntriesForOfPair() {
  let n = 0;
  for (const ea of new Map([["k", new A()]]).entries()) {
    n += ea[1].run();
  }
  for (const eb of new Map([["k", new B()]]).entries()) {
    n += eb[1].run();
  }
  return n;
}

function useMapEntriesLocal() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  let n = 0;
  for (const [, xa] of ma.entries()) {
    n += xa.run();
  }
  for (const [, xb] of mb.entries()) {
    n += xb.run();
  }
  return n;
}

function useMapForOfDirect() {
  let n = 0;
  for (const [, xa] of new Map([["k", new A()]])) {
    n += xa.run();
  }
  for (const [, xb] of new Map([["k", new B()]])) {
    n += xb.run();
  }
  return n;
}

function useMapEntriesSpread() {
  return (
    [...new Map([["k", new A()]]).entries()][0][1].run() +
    [...new Map([["k", new B()]]).entries()][0][1].run()
  );
}

function useMapEntriesNextValuePair() {
  return (
    new Map([["k", new A()]]).entries().next().value[1].run() +
    new Map([["k", new B()]]).entries().next().value[1].run()
  );
}

function useMapEntriesLocalNext() {
  const ia = new Map([["k", new A()]]).entries();
  const ib = new Map([["k", new B()]]).entries();
  return ia.next().value[1].run() + ib.next().value[1].run();
}

function useIdent() {
  const a = new A();
  const b = new B();
  let n = 0;
  for (const [, xa] of new Map([["k", a]]).entries()) {
    n += xa.run();
  }
  for (const [, xb] of new Map([["k", b]]).entries()) {
    n += xb.run();
  }
  return n;
}

function useMultiPair() {
  return (
    [...new Map([["k", new A()], ["j", new A()]]).entries()][1][1].run() +
    [...new Map([["k", new B()], ["j", new B()]]).entries()][1][1].run()
  );
}


function useMapValuesNext() {
  return (
    new Map([["k", new A()]]).values().next().value.run() +
    new Map([["k", new B()]]).values().next().value.run()
  );
}

function useMapValuesForOf() {
  let n = 0;
  for (const xa of new Map([["k", new A()]]).values()) {
    n += xa.run();
  }
  for (const xb of new Map([["k", new B()]]).values()) {
    n += xb.run();
  }
  return n;
}

function useMapGet() {
  return (
    new Map([["k", new A()]]).get("k").run() +
    new Map([["k", new B()]]).get("k").run()
  );
}

function useMapLocalGet() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  return ma.get("k").run() + mb.get("k").run();
}

function useMapGetAssign() {
  const xa = new Map([["k", new A()]]).get("k");
  const xb = new Map([["k", new B()]]).get("k");
  return xa.run() + xb.run();
}

function usePreservesB() {
  let n = 0;
  for (const [, xb] of new Map([["k", new B()]]).entries()) {
    n += xb.run();
  }
  for (const eb of new Map([["k", new B()]])) {
    n += eb[1].run();
  }
  return (
    n +
    [...new Map([["k", new B()]]).entries()][0][1].run() +
    new Map([["k", new B()]]).entries().next().value[1].run() +
    new Map([["k", new B()]]).values().next().value.run() +
    new Map([["k", new B()]]).get("k").run()
  );
}
