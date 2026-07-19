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

function useAssignLocalInline() {
  const oa = Object.assign({}, { k: new A() });
  const ob = Object.assign({}, { k: new B() });
  return Object.entries(oa)[0][1].execute() + Object.entries(ob)[0][1].run();
}

function useAssignInline() {
  return (
    Object.entries(Object.assign({}, { k: new A() }))[0][1].execute() +
    Object.entries(Object.assign({}, { k: new B() }))[0][1].run()
  );
}

function useEntriesLocal() {
  const oa = Object.assign({}, { k: new A() });
  const ob = Object.assign({}, { k: new B() });
  const ea = Object.entries(oa);
  const eb = Object.entries(ob);
  return ea[0][1].execute() + eb[0][1].run();
}

function usePairLocal() {
  const oa = Object.assign({}, { k: new A() });
  const ob = Object.assign({}, { k: new B() });
  const ea = Object.entries(oa)[0];
  const eb = Object.entries(ob)[0];
  return ea[1].execute() + eb[1].run();
}

function useForOf() {
  let n = 0;
  const oa = Object.assign({}, { k: new A() });
  const ob = Object.assign({}, { k: new B() });
  for (const ea of Object.entries(oa)) {
    n += ea[1].execute();
  }
  for (const eb of Object.entries(ob)) {
    n += eb[1].run();
  }
  return n;
}

function useDestructure() {
  const oa = Object.assign({}, { k: new A() });
  const ob = Object.assign({}, { k: new B() });
  const [, a] = Object.entries(oa)[0];
  const [, b] = Object.entries(ob)[0];
  return a.execute() + b.run();
}

function useFromEntriesLocal() {
  const oa = Object.fromEntries([["k", new A()]]);
  const ob = Object.fromEntries([["k", new B()]]);
  return Object.entries(oa)[0][1].execute() + Object.entries(ob)[0][1].run();
}

function useIdent() {
  const a0 = new A();
  const b0 = new B();
  const oa = Object.assign({}, { k: a0 });
  const ob = Object.assign({}, { k: b0 });
  return Object.entries(oa)[0][1].execute() + Object.entries(ob)[0][1].run();
}

function usePreservesB() {
  const o = Object.assign({}, { k: new B() });
  return Object.entries(o)[0][1].run();
}
