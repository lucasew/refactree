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

function useDestructure() {
  const { k: a } = { k: new A() };
  const { k: b } = { k: new B() };
  return a.run() + b.run();
}

function useShorthand() {
  const a0 = new A();
  const b0 = new B();
  const { a0: a } = { a0 };
  const { b0: b } = { b0 };
  return a.run() + b.run();
}

function useFromLocal() {
  const oa = { k: new A() };
  const ob = { k: new B() };
  const { k: a } = oa;
  const { k: b } = ob;
  return a.run() + b.run();
}

function useSpreadDestructure() {
  const { k: a } = { ...{ k: new A() } };
  const { k: b } = { ...{ k: new B() } };
  return a.run() + b.run();
}
