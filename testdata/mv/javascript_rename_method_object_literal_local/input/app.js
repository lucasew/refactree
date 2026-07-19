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

function useValuesLocal() {
  const oa = { k: new A() };
  const ob = { k: new B() };
  return Object.values(oa)[0].run() + Object.values(ob)[0].run();
}

function usePropLocal() {
  const oa = { k: new A() };
  const ob = { k: new B() };
  return oa.k.run() + ob.k.run();
}

function useValuesVar() {
  const oa = { k: new A() };
  const ob = { k: new B() };
  const a = Object.values(oa)[0];
  const b = Object.values(ob)[0];
  return a.run() + b.run();
}

function useShorthand() {
  const a = new A();
  const b = new B();
  const oa = { a };
  const ob = { b };
  return Object.values(oa)[0].run() + Object.values(ob)[0].run();
}

function usePreservesB() {
  const ob = { k: new B() };
  return Object.values(ob)[0].run() + ob.k.run();
}
