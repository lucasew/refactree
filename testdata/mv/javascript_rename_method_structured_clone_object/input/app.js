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

function useCloneProp() {
  return structuredClone({ k: new A() }).k.run() + structuredClone({ k: new B() }).k.run();
}

function useCloneLocal() {
  const oa = { k: new A() };
  const ob = { k: new B() };
  return structuredClone(oa).k.run() + structuredClone(ob).k.run();
}

function useCloneAssign() {
  const a = structuredClone({ k: new A() }).k;
  const b = structuredClone({ k: new B() }).k;
  return a.run() + b.run();
}

function useCloneBracket() {
  return structuredClone({ k: new A() })["k"].run() + structuredClone({ k: new B() })["k"].run();
}
