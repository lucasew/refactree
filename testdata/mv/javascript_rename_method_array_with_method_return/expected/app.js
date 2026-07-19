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

function useWith() {
  return (
    [new BoxA().get()].with(0, new BoxA().get())[0].execute() +
    [new BoxB().get()].with(0, new BoxB().get())[0].run()
  );
}

function useWithEmpty() {
  return (
    [].with(0, new BoxA().get())[0].execute() +
    [].with(0, new BoxB().get())[0].run()
  );
}

function useWithAssign() {
  const xs = [].with(0, new BoxA().get());
  const ys = [].with(0, new BoxB().get());
  return xs[0].execute() + ys[0].run();
}

function useClass() {
  return (
    [new A()].with(0, new A())[0].execute() +
    [new B()].with(0, new B())[0].run() +
    [].with(0, new A())[0].execute() +
    [].with(0, new B())[0].run()
  );
}

function usePreservesB() {
  return (
    [].with(0, new BoxB().get())[0].run() +
    [new BoxB().get()].with(0, new BoxB().get())[0].run()
  );
}
