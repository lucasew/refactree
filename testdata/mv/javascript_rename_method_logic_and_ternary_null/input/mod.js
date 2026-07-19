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

function useAnd() {
  return (true && new A()).run() + (true && new B()).run();
}

function useAndLocal() {
  const a = true && new A();
  const b = true && new B();
  return a.run() + b.run();
}

function useTernaryNull() {
  return (true ? new A() : null).run() + (true ? new B() : null).run();
}

function useTernaryUndefined() {
  return (true ? new A() : undefined).run() + (true ? new B() : undefined).run();
}

function useTernaryNullLocal() {
  const a = true ? new A() : null;
  const b = true ? new B() : null;
  return a.run() + b.run();
}

function useOrQQStill() {
  return (null || new A()).run() + (null ?? new B()).run();
}

function usePreservesB() {
  return (true && new B()).run() + (true ? new B() : null).run();
}
