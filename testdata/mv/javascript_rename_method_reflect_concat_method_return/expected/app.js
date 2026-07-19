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

function useReflect() {
  return (
    Reflect.get({ k: new BoxA().get() }, "k").execute() +
    Reflect.get({ k: new BoxB().get() }, "k").run()
  );
}

function useReflectAssign() {
  const xa = Reflect.get({ k: new BoxA().get() }, "k");
  const xb = Reflect.get({ k: new BoxB().get() }, "k");
  return xa.execute() + xb.run();
}

function useConcat() {
  return (
    [].concat(new BoxA().get())[0].execute() +
    [].concat(new BoxB().get())[0].run() +
    [new BoxA().get()].concat([])[0].execute() +
    [new BoxB().get()].concat([])[0].run()
  );
}

function useConcatAssign() {
  const xs = [].concat(new BoxA().get());
  const ys = [].concat(new BoxB().get());
  return xs[0].execute() + ys[0].run();
}

function useClass() {
  return (
    Reflect.get({ k: new A() }, "k").execute() +
    Reflect.get({ k: new B() }, "k").run() +
    [].concat(new A())[0].execute() +
    [].concat(new B())[0].run() +
    [new A()].concat([])[0].execute() +
    [new B()].concat([])[0].run()
  );
}

function usePreservesB() {
  return (
    Reflect.get({ k: new BoxB().get() }, "k").run() +
    [].concat(new BoxB().get())[0].run()
  );
}
