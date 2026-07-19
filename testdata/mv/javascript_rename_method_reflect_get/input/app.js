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

function useInline() {
  return Reflect.get({ k: new A() }, "k").run() + Reflect.get({ k: new B() }, "k").run();
}

function useLocal() {
  const oa = { k: new A() };
  const ob = { k: new B() };
  return Reflect.get(oa, "k").run() + Reflect.get(ob, "k").run();
}

function useAssign() {
  const a = Reflect.get({ k: new A() }, "k");
  const b = Reflect.get({ k: new B() }, "k");
  return a.run() + b.run();
}

function usePreservesB() {
  return Reflect.get({ k: new B() }, "k").run();
}
