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

function useNullWith() {
  return [null].with(0, new A())[0].run() + [null].with(0, new B())[0].run();
}

function useUndefinedWith() {
  return (
    [undefined].with(0, new A())[0].run() + [undefined].with(0, new B())[0].run()
  );
}

function useAssign() {
  const aa = [null].with(0, new A());
  const bb = [null].with(0, new B());
  return aa[0].run() + bb[0].run();
}

function useEmptyStill() {
  return [].with(0, new A())[0].run() + [].with(0, new B())[0].run();
}

function usePreservesB() {
  return [null].with(0, new B())[0].run();
}
