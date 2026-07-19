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

function makeA() {
  return new A();
}

function makeB() {
  return new B();
}

function useStructuredClone() {
  return structuredClone(new A()).run() + structuredClone(new B()).run();
}

function useObjectAssign() {
  return Object.assign(new A()).run() + Object.assign(new B()).run();
}

function useObjectAssignSources() {
  return (
    Object.assign(new A(), { x: 1 }).run() + Object.assign(new B(), { x: 2 }).run()
  );
}

function useIdent() {
  const a = new A();
  const b = new B();
  return structuredClone(a).run() + structuredClone(b).run();
}

function useFactory() {
  return (
    structuredClone(makeA()).run() +
    structuredClone(makeB()).run() +
    Object.assign(makeA()).run() +
    Object.assign(makeB()).run()
  );
}

function useAssignLocal() {
  const a = structuredClone(new A());
  const b = Object.assign(new B());
  return a.run() + b.run();
}

function usePreservesB() {
  return structuredClone(new B()).run() + Object.assign(new B()).run();
}
