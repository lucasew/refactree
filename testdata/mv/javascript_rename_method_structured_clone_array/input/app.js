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

function useCloneArrayLocal() {
  const aa = [new A()];
  const bb = [new B()];
  return structuredClone(aa)[0].run() + structuredClone(bb)[0].run();
}

function useCloneArrayInline() {
  return (
    structuredClone([new A()])[0].run() + structuredClone([new B()])[0].run()
  );
}

function useCloneArrayAssign() {
  const aa = [new A()];
  const bb = [new B()];
  const ca = structuredClone(aa);
  const cb = structuredClone(bb);
  return ca[0].run() + cb[0].run();
}
