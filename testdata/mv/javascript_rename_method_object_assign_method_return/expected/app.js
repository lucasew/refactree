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

function useMember() {
  return (
    Object.assign({}, { k: new BoxA().get() }).k.execute() +
    Object.assign({}, { k: new BoxB().get() }).k.run()
  );
}

function useValues() {
  return (
    Object.values(Object.assign({}, { k: new BoxA().get() }))[0].execute() +
    Object.values(Object.assign({}, { k: new BoxB().get() }))[0].run()
  );
}

function useLocal() {
  const oa = Object.assign({}, { k: new BoxA().get() });
  const ob = Object.assign({}, { k: new BoxB().get() });
  return oa.k.execute() + ob.k.run();
}

function useIdent() {
  const ba = new BoxA();
  const bb = new BoxB();
  return (
    Object.assign({}, { k: ba.get() }).k.execute() +
    Object.assign({}, { k: bb.get() }).k.run()
  );
}

function useClass() {
  return (
    Object.assign({}, { k: new A() }).k.execute() +
    Object.assign({}, { k: new B() }).k.run()
  );
}

function usePreservesB() {
  const ob = Object.assign({}, { k: new BoxB().get() });
  return (
    Object.assign({}, { k: new BoxB().get() }).k.run() +
    ob.k.run()
  );
}
