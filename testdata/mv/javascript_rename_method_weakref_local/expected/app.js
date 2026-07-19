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

function useLocal() {
  const wa = new WeakRef(new A());
  const wb = new WeakRef(new B());
  return wa.deref().execute() + wb.deref().run();
}

function useLocalTyped() {
  const a = new A();
  const b = new B();
  const wa = new WeakRef(a);
  const wb = new WeakRef(b);
  return wa.deref().execute() + wb.deref().run();
}

function useAssign() {
  const wa = new WeakRef(new A());
  const wb = new WeakRef(new B());
  const xa = wa.deref();
  const xb = wb.deref();
  return xa.execute() + xb.run();
}

function usePreservesB() {
  const wb = new WeakRef(new B());
  return wb.deref().run();
}
