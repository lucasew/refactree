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

function useFreeze() {
  const a = new A();
  const b = new B();
  return Object.freeze(a).execute() + Object.freeze(b).run();
}

function useSeal() {
  return Object.seal(new A()).execute() + Object.seal(new B()).run();
}

function usePreventExtensions() {
  return (
    Object.preventExtensions(new A()).execute() +
    Object.preventExtensions(new B()).run()
  );
}

function useFreezeAssign() {
  const a = Object.freeze(new A());
  const b = Object.freeze(new B());
  return a.execute() + b.run();
}

function useSealAssign() {
  const a = new A();
  const b = new B();
  const fa = Object.seal(a);
  const fb = Object.seal(b);
  return fa.execute() + fb.run();
}
