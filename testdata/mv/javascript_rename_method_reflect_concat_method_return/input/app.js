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

function useReflect() {
  return (
    Reflect.get({ k: new BoxA().get() }, "k").run() +
    Reflect.get({ k: new BoxB().get() }, "k").run()
  );
}

function useReflectAssign() {
  const xa = Reflect.get({ k: new BoxA().get() }, "k");
  const xb = Reflect.get({ k: new BoxB().get() }, "k");
  return xa.run() + xb.run();
}

function useConcat() {
  return (
    [].concat(new BoxA().get())[0].run() +
    [].concat(new BoxB().get())[0].run() +
    [new BoxA().get()].concat([])[0].run() +
    [new BoxB().get()].concat([])[0].run()
  );
}

function useConcatAssign() {
  const xs = [].concat(new BoxA().get());
  const ys = [].concat(new BoxB().get());
  return xs[0].run() + ys[0].run();
}

function useClass() {
  return (
    Reflect.get({ k: new A() }, "k").run() +
    Reflect.get({ k: new B() }, "k").run() +
    [].concat(new A())[0].run() +
    [].concat(new B())[0].run() +
    [new A()].concat([])[0].run() +
    [new B()].concat([])[0].run()
  );
}

function usePreservesB() {
  return (
    Reflect.get({ k: new BoxB().get() }, "k").run() +
    [].concat(new BoxB().get())[0].run()
  );
}
