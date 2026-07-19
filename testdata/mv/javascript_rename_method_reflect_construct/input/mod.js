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

function useConstruct() {
  return Reflect.construct(A, []).run() + Reflect.construct(B, []).run();
}

function useConstructLocal() {
  const a = Reflect.construct(A, []);
  const b = Reflect.construct(B, []);
  return a.run() + b.run();
}

function useConstructArgs() {
  return Reflect.construct(A, [], A).run() + Reflect.construct(B, [], B).run();
}

function usePreservesB() {
  return Reflect.construct(B, []).run();
}
