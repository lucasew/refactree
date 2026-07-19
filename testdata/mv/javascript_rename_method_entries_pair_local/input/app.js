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

function useEntryLocalIndex() {
  const ea = Object.entries({ k: new A() })[0];
  const eb = Object.entries({ k: new B() })[0];
  return ea[1].run() + eb[1].run();
}

function useEntryLocalForOf() {
  let n = 0;
  for (const ea of Object.entries({ k: new A() })) {
    n += ea[1].run();
  }
  for (const eb of Object.entries({ k: new B() })) {
    n += eb[1].run();
  }
  return n;
}

function useEntryLocalValueAssign() {
  const ea = Object.entries({ k: new A() })[0];
  const eb = Object.entries({ k: new B() })[0];
  const a = ea[1];
  const b = eb[1];
  return a.run() + b.run();
}

function useEntryLocalDestructure() {
  const ea = Object.entries({ k: new A() })[0];
  const eb = Object.entries({ k: new B() })[0];
  const [, a] = ea;
  const [, b] = eb;
  return a.run() + b.run();
}

function useIdent() {
  const a = new A();
  const b = new B();
  const ea = Object.entries({ k: a })[0];
  const eb = Object.entries({ k: b })[0];
  return ea[1].run() + eb[1].run();
}

function useShorthand() {
  const a = new A();
  const b = new B();
  const ea = Object.entries({ a })[0];
  const eb = Object.entries({ b })[0];
  return ea[1].run() + eb[1].run();
}

function usePreservesB() {
  const eb = Object.entries({ k: new B() })[0];
  let n = eb[1].run();
  for (const x of Object.entries({ k: new B() })) {
    n += x[1].run();
  }
  return n;
}
