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

function useFreeze() {
  const a = new A();
  const b = new B();
  return Object.freeze(a).run() + Object.freeze(b).run();
}

function useSeal() {
  return Object.seal(new A()).run() + Object.seal(new B()).run();
}

function usePreventExtensions() {
  return (
    Object.preventExtensions(new A()).run() +
    Object.preventExtensions(new B()).run()
  );
}

function useFreezeAssign() {
  const a = Object.freeze(new A());
  const b = Object.freeze(new B());
  return a.run() + b.run();
}

function useSealAssign() {
  const a = new A();
  const b = new B();
  const fa = Object.seal(a);
  const fb = Object.seal(b);
  return fa.run() + fb.run();
}
