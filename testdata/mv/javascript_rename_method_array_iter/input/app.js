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

function useForOfLiteral() {
  let n = 0;
  for (const a of [new A()]) {
    n += a.run();
  }
  for (const b of [new B()]) {
    n += b.run();
  }
  return n;
}

function useForOfLocal() {
  const as = [new A()];
  const bs = [new B()];
  let n = 0;
  for (const a of as) {
    n += a.run();
  }
  for (const b of bs) {
    n += b.run();
  }
  return n;
}

function useValuesNextValue() {
  return (
    [new A()].values().next().value.run() +
    [new B()].values().next().value.run()
  );
}

function useValuesNextAssign() {
  const ra = [new A()].values().next();
  const rb = [new B()].values().next();
  return ra.value.run() + rb.value.run();
}

function useValuesLocal() {
  const ia = [new A()].values();
  const ib = [new B()].values();
  return ia.next().value.run() + ib.next().value.run();
}

function useSymbolIterator() {
  return (
    [new A()][Symbol.iterator]().next().value.run() +
    [new B()][Symbol.iterator]().next().value.run()
  );
}

function useSymbolIteratorLocal() {
  const ia = [new A()][Symbol.iterator]();
  const ib = [new B()][Symbol.iterator]();
  return ia.next().value.run() + ib.next().value.run();
}

function useIdentArray() {
  const a = new A();
  const b = new B();
  let n = 0;
  for (const xa of [a]) {
    n += xa.run();
  }
  for (const xb of [b]) {
    n += xb.run();
  }
  return (
    n +
    [a].values().next().value.run() +
    [b].values().next().value.run()
  );
}

function usePreservesB() {
  let n = 0;
  for (const b of [new B()]) {
    n += b.run();
  }
  return n + [new B()].values().next().value.run();
}
