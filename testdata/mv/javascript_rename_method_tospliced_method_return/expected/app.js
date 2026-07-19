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

function useToSpliced() {
  return (
    [null].toSpliced(0, 1, new BoxA().get())[0].execute() +
    [null].toSpliced(0, 1, new BoxB().get())[0].run()
  );
}

function useToSplicedAssign() {
  const xa = [null].toSpliced(0, 1, new BoxA().get())[0];
  const xb = [null].toSpliced(0, 1, new BoxB().get())[0];
  return xa.execute() + xb.run();
}

function useClass() {
  return (
    [null].toSpliced(0, 1, new A())[0].execute() +
    [null].toSpliced(0, 1, new B())[0].run()
  );
}

function usePreservesB() {
  return (
    [null].toSpliced(0, 1, new BoxB().get())[0].run() +
    [null].toSpliced(0, 1, new B())[0].run()
  );
}
