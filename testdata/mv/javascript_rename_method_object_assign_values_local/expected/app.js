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

function useAssignLocalValues() {
  const oa = Object.assign({}, { k: new A() });
  const ob = Object.assign({}, { k: new B() });
  return Object.values(oa)[0].execute() + Object.values(ob)[0].run();
}

function useAssignLocalForOf() {
  let n = 0;
  const oa = Object.assign({}, { k: new A() });
  const ob = Object.assign({}, { k: new B() });
  for (const xa of Object.values(oa)) {
    n += xa.execute();
  }
  for (const xb of Object.values(ob)) {
    n += xb.run();
  }
  return n;
}

function useAssignLocalValuesLocal() {
  const oa = Object.assign({}, { k: new A() });
  const ob = Object.assign({}, { k: new B() });
  const av = Object.values(oa);
  const bv = Object.values(ob);
  return av[0].execute() + bv[0].run();
}

function useFromEntriesLocalValues() {
  const oa = Object.fromEntries([["k", new A()]]);
  const ob = Object.fromEntries([["k", new B()]]);
  return Object.values(oa)[0].execute() + Object.values(ob)[0].run();
}

function useIdent() {
  const a0 = new A();
  const b0 = new B();
  const oa = Object.assign({}, { k: a0 });
  const ob = Object.assign({}, { k: b0 });
  return Object.values(oa)[0].execute() + Object.values(ob)[0].run();
}

function usePreservesB() {
  const o = Object.assign({}, { k: new B() });
  return Object.values(o)[0].run();
}
