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

function useConstruct() {
  return Reflect.construct(A, []).execute() + Reflect.construct(B, []).run();
}

function useConstructLocal() {
  const a = Reflect.construct(A, []);
  const b = Reflect.construct(B, []);
  return a.execute() + b.run();
}

function useConstructArgs() {
  return Reflect.construct(A, [], A).execute() + Reflect.construct(B, [], B).run();
}

function usePreservesB() {
  return Reflect.construct(B, []).run();
}
