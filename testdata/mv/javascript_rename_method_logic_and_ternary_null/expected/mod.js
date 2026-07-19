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

function useAnd() {
  return (true && new A()).execute() + (true && new B()).run();
}

function useAndLocal() {
  const a = true && new A();
  const b = true && new B();
  return a.execute() + b.run();
}

function useTernaryNull() {
  return (true ? new A() : null).execute() + (true ? new B() : null).run();
}

function useTernaryUndefined() {
  return (true ? new A() : undefined).execute() + (true ? new B() : undefined).run();
}

function useTernaryNullLocal() {
  const a = true ? new A() : null;
  const b = true ? new B() : null;
  return a.execute() + b.run();
}

function useOrQQStill() {
  return (null || new A()).execute() + (null ?? new B()).run();
}

function usePreservesB() {
  return (true && new B()).run() + (true ? new B() : null).run();
}
