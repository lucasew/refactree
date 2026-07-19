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

class BoxA {
  a = new A();
  get() {
    return this.a;
  }
}

class BoxB {
  b = new B();
  get() {
    return this.b;
  }
}

function useStructuredClone() {
  return (
    structuredClone({ k: new BoxA().get() }).k.execute() +
    structuredClone({ k: new BoxB().get() }).k.run()
  );
}

function useStructuredCloneAssign() {
  const xa = structuredClone({ k: new BoxA().get() }).k;
  const xb = structuredClone({ k: new BoxB().get() }).k;
  return xa.execute() + xb.run();
}

function useStructuredCloneBracket() {
  return (
    structuredClone({ k: new BoxA().get() })["k"].execute() +
    structuredClone({ k: new BoxB().get() })["k"].run()
  );
}

function useClass() {
  return (
    structuredClone({ k: new A() }).k.execute() +
    structuredClone({ k: new B() }).k.run()
  );
}

function usePreservesB() {
  return (
    structuredClone({ k: new BoxB().get() }).k.run() +
    structuredClone({ k: new B() }).k.run()
  );
}
