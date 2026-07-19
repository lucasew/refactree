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

function useInline() {
  return new WeakRef(new A()).deref().execute() + new WeakRef(new B()).deref().run();
}

function useAssign() {
  const xa = new WeakRef(new A()).deref();
  const xb = new WeakRef(new B()).deref();
  return xa.execute() + xb.run();
}

function usePreservesB() {
  return new WeakRef(new B()).deref().run();
}
