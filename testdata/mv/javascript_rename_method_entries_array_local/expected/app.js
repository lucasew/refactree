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

function useEntriesArrayLocal() {
  const esa = Object.entries({ k: new A() });
  const esb = Object.entries({ k: new B() });
  return esa[0][1].execute() + esb[0][1].run();
}

function useEntriesArrayLocalValue() {
  const esa = Object.entries({ k: new A() });
  const esb = Object.entries({ k: new B() });
  const a = esa[0][1];
  const b = esb[0][1];
  return a.execute() + b.run();
}

function useEntriesArrayLocalPair() {
  const esa = Object.entries({ k: new A() });
  const esb = Object.entries({ k: new B() });
  const ea = esa[0];
  const eb = esb[0];
  return ea[1].execute() + eb[1].run();
}

function useEntriesArrayLocalForOf() {
  const esa = Object.entries({ k: new A() });
  const esb = Object.entries({ k: new B() });
  let n = 0;
  for (const ea of esa) {
    n += ea[1].execute();
  }
  for (const eb of esb) {
    n += eb[1].run();
  }
  return n;
}

function useEntriesArrayLocalForOfDestructure() {
  const esa = Object.entries({ k: new A() });
  const esb = Object.entries({ k: new B() });
  let n = 0;
  for (const [, a] of esa) {
    n += a.execute();
  }
  for (const [, b] of esb) {
    n += b.run();
  }
  return n;
}

function useEntriesSpread() {
  return (
    [...Object.entries({ k: new A() })][0][1].execute() +
    [...Object.entries({ k: new B() })][0][1].run()
  );
}

function useEntriesSpreadLocal() {
  const esa = [...Object.entries({ k: new A() })];
  const esb = [...Object.entries({ k: new B() })];
  return esa[0][1].execute() + esb[0][1].run();
}

function useArrayEntriesForOf() {
  let n = 0;
  for (const [, a] of [new A()].entries()) {
    n += a.execute();
  }
  for (const [, b] of [new B()].entries()) {
    n += b.run();
  }
  return n;
}

function useArrayEntriesForOfPair() {
  let n = 0;
  for (const ea of [new A()].entries()) {
    n += ea[1].execute();
  }
  for (const eb of [new B()].entries()) {
    n += eb[1].run();
  }
  return n;
}

function useArrayEntriesLocalForOf() {
  const as = [new A()];
  const bs = [new B()];
  let n = 0;
  for (const [, a] of as.entries()) {
    n += a.execute();
  }
  for (const [, b] of bs.entries()) {
    n += b.run();
  }
  return n;
}

function useIdent() {
  const a = new A();
  const b = new B();
  const esa = Object.entries({ k: a });
  const esb = Object.entries({ k: b });
  return esa[0][1].execute() + esb[0][1].run();
}

function useShorthand() {
  const a = new A();
  const b = new B();
  const esa = Object.entries({ a });
  const esb = Object.entries({ b });
  return esa[0][1].execute() + esb[0][1].run();
}

function usePreservesB() {
  const es = Object.entries({ k: new B() });
  let n = es[0][1].run() + [...Object.entries({ k: new B() })][0][1].run();
  for (const [, b] of [new B()].entries()) {
    n += b.run();
  }
  return n;
}
