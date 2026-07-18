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

function useValuesNextValue() {
  return (
    new Set([new A()]).values().next().value.run() +
    new Set([new B()]).values().next().value.run()
  );
}

function useKeysNextValue() {
  return (
    new Set([new A()]).keys().next().value.run() +
    new Set([new B()]).keys().next().value.run()
  );
}

function useSymbolIterator() {
  return (
    new Set([new A()])[Symbol.iterator]().next().value.run() +
    new Set([new B()])[Symbol.iterator]().next().value.run()
  );
}

function useValuesLocal() {
  const ia = new Set([new A()]).values();
  const ib = new Set([new B()]).values();
  return ia.next().value.run() + ib.next().value.run();
}

function useKeysLocal() {
  const ia = new Set([new A()]).keys();
  const ib = new Set([new B()]).keys();
  return ia.next().value.run() + ib.next().value.run();
}

function useSetLocalValues() {
  const sa = new Set([new A()]);
  const sb = new Set([new B()]);
  return sa.values().next().value.run() + sb.values().next().value.run();
}

function useSetLocalKeys() {
  const sa = new Set([new A()]);
  const sb = new Set([new B()]);
  return sa.keys().next().value.run() + sb.keys().next().value.run();
}

function useValuesNextAssign() {
  const ra = new Set([new A()]).values().next();
  const rb = new Set([new B()]).values().next();
  return ra.value.run() + rb.value.run();
}

function useValuesForOf() {
  let n = 0;
  for (const xa of new Set([new A()]).values()) {
    n += xa.run();
  }
  for (const xb of new Set([new B()]).values()) {
    n += xb.run();
  }
  return n;
}

function useKeysForOfLocal() {
  const sa = new Set([new A()]);
  const sb = new Set([new B()]);
  let n = 0;
  for (const xa of sa.keys()) {
    n += xa.run();
  }
  for (const xb of sb.keys()) {
    n += xb.run();
  }
  return n;
}

function useArrayLocal() {
  const as = [new A()];
  const bs = [new B()];
  return (
    new Set(as).values().next().value.run() +
    new Set(bs).keys().next().value.run()
  );
}

function useIdent() {
  const a = new A();
  const b = new B();
  return (
    new Set([a]).values().next().value.run() +
    new Set([b]).keys().next().value.run()
  );
}

function usePreservesB() {
  const sb = new Set([new B()]);
  let n = 0;
  for (const xb of sb.values()) {
    n += xb.run();
  }
  return (
    n +
    new Set([new B()]).values().next().value.run() +
    new Set([new B()]).keys().next().value.run() +
    sb.keys().next().value.run()
  );
}
