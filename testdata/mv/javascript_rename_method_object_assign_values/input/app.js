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

function useAssignValues() {
  return (
    Object.values(Object.assign({}, { k: new A() }))[0].run() +
    Object.values(Object.assign({}, { k: new B() }))[0].run()
  );
}

function useAssignValuesTarget() {
  return (
    Object.values(Object.assign({ k: new A() }))[0].run() +
    Object.values(Object.assign({ k: new B() }))[0].run()
  );
}

function useAssignValuesMultiSame() {
  return (
    Object.values(Object.assign({}, { k: new A() }, { j: new A() }))[0].run() +
    Object.values(Object.assign({}, { k: new B() }, { j: new B() }))[0].run()
  );
}

function useAssignValuesForOf() {
  let n = 0;
  for (const xa of Object.values(Object.assign({}, { k: new A() }))) {
    n += xa.run();
  }
  for (const xb of Object.values(Object.assign({}, { k: new B() }))) {
    n += xb.run();
  }
  return n;
}

function useAssignValuesLocal() {
  const av = Object.values(Object.assign({}, { k: new A() }));
  const bv = Object.values(Object.assign({}, { k: new B() }));
  return av[0].run() + bv[0].run();
}

function useIdent() {
  const a0 = new A();
  const b0 = new B();
  return (
    Object.values(Object.assign({}, { k: a0 }))[0].run() +
    Object.values(Object.assign({}, { k: b0 }))[0].run()
  );
}

function useShorthand() {
  const a0 = new A();
  const b0 = new B();
  return (
    Object.values(Object.assign({}, { a0 }))[0].run() +
    Object.values(Object.assign({}, { b0 }))[0].run()
  );
}

function usePreservesB() {
  return Object.values(Object.assign({}, { k: new B() }))[0].run();
}
