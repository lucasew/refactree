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

function useEntriesNextValuePair() {
  return (
    new Set([new A()]).entries().next().value[1].run() +
    new Set([new B()]).entries().next().value[1].run()
  );
}

function useEntriesNextValueLocal() {
  const ea = new Set([new A()]).entries().next().value;
  const eb = new Set([new B()]).entries().next().value;
  return ea[1].run() + eb[1].run();
}

function useEntriesNextLocal() {
  const ra = new Set([new A()]).entries().next();
  const rb = new Set([new B()]).entries().next();
  return ra.value[1].run() + rb.value[1].run();
}

function useEntriesNextValueDestructure() {
  const [, xa] = new Set([new A()]).entries().next().value;
  const [, xb] = new Set([new B()]).entries().next().value;
  return xa.run() + xb.run();
}

function useEntriesLocalNext() {
  const ia = new Set([new A()]).entries();
  const ib = new Set([new B()]).entries();
  return ia.next().value[1].run() + ib.next().value[1].run();
}

function useSetLocalEntries() {
  const sa = new Set([new A()]);
  const sb = new Set([new B()]);
  return sa.entries().next().value[1].run() + sb.entries().next().value[1].run();
}

function useEntriesForOfDestructure() {
  let n = 0;
  for (const [, xa] of new Set([new A()]).entries()) {
    n += xa.run();
  }
  for (const [, xb] of new Set([new B()]).entries()) {
    n += xb.run();
  }
  return n;
}

function useEntriesForOfPair() {
  let n = 0;
  for (const ea of new Set([new A()]).entries()) {
    n += ea[1].run();
  }
  for (const eb of new Set([new B()]).entries()) {
    n += eb[1].run();
  }
  return n;
}

function useEntriesForOfLocal() {
  const sa = new Set([new A()]);
  const sb = new Set([new B()]);
  let n = 0;
  for (const [, xa] of sa.entries()) {
    n += xa.run();
  }
  for (const [, xb] of sb.entries()) {
    n += xb.run();
  }
  return n;
}

function useEntriesSpread() {
  return (
    [...new Set([new A()]).entries()][0][1].run() +
    [...new Set([new B()]).entries()][0][1].run()
  );
}

function useIdent() {
  const a = new A();
  const b = new B();
  return (
    new Set([a]).entries().next().value[1].run() +
    new Set([b]).entries().next().value[1].run()
  );
}

function useArrayLocal() {
  const as = [new A()];
  const bs = [new B()];
  return (
    new Set(as).entries().next().value[1].run() +
    new Set(bs).entries().next().value[1].run()
  );
}

function usePreservesB() {
  const eb = new Set([new B()]).entries().next().value;
  const rb = new Set([new B()]).entries().next();
  return (
    new Set([new B()]).entries().next().value[1].run() +
    eb[1].run() +
    rb.value[1].run()
  );
}
