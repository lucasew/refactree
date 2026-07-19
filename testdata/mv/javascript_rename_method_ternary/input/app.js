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

function useCtor(c) {
  return (c ? new A() : new A()).run() + (c ? new B() : new B()).run();
}

function useLocal(c) {
  const a = new A();
  const x = new A();
  const b = new B();
  const y = new B();
  return (c ? a : x).run() + (c ? b : y).run();
}

function useAssign(c) {
  const a = new A();
  const x = new A();
  const b = new B();
  const y = new B();
  const xa = c ? a : x;
  const xb = c ? b : y;
  return xa.run() + xb.run();
}
