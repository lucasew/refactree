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

function useFromMapfnCtor() {
  return (
    Array.from([0], () => new BoxA().get())[0].execute() +
    Array.from([0], () => new BoxB().get())[0].run()
  );
}

function useFromMapfnCtorAssign() {
  const xs = Array.from([0], () => new BoxA().get());
  const ys = Array.from([0], () => new BoxB().get());
  return xs[0].execute() + ys[0].run();
}

function useFromMapfnCtorAt() {
  return (
    Array.from([0], (x) => new BoxA().get()).at(0).execute() +
    Array.from([0], (x) => new BoxB().get()).at(0).run()
  );
}

function useFromMapfnLength() {
  return (
    Array.from({ length: 1 }, () => new BoxA().get())[0].execute() +
    Array.from({ length: 1 }, () => new BoxB().get())[0].run()
  );
}

function useFromMapfnBlock() {
  return (
    Array.from([0], () => {
      return new BoxA().get();
    })[0].execute() +
    Array.from([0], () => {
      return new BoxB().get();
    })[0].run()
  );
}

// Class regression — already worked.
function useClassFromMapfn() {
  return (
    Array.from([0], () => new A())[0].execute() +
    Array.from([0], () => new B())[0].run() +
    Array.from({ length: 1 }, () => new A())[0].execute() +
    Array.from({ length: 1 }, () => new B())[0].run()
  );
}

function usePreservesB() {
  return (
    Array.from([0], () => new BoxB().get())[0].run() +
    Array.from({ length: 1 }, () => new BoxB().get())[0].run()
  );
}
