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

function makeA() {
  return new A();
}

function makeB() {
  return new B();
}

function useStructuredClone() {
  return structuredClone(new A()).execute() + structuredClone(new B()).run();
}

function useObjectAssign() {
  return Object.assign(new A()).execute() + Object.assign(new B()).run();
}

function useObjectAssignSources() {
  return (
    Object.assign(new A(), { x: 1 }).execute() + Object.assign(new B(), { x: 2 }).run()
  );
}

function useIdent() {
  const a = new A();
  const b = new B();
  return structuredClone(a).execute() + structuredClone(b).run();
}

function useFactory() {
  return (
    structuredClone(makeA()).execute() +
    structuredClone(makeB()).run() +
    Object.assign(makeA()).execute() +
    Object.assign(makeB()).run()
  );
}

function useAssignLocal() {
  const a = structuredClone(new A());
  const b = Object.assign(new B());
  return a.execute() + b.run();
}

function usePreservesB() {
  return structuredClone(new B()).run() + Object.assign(new B()).run();
}
