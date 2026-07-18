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

function useWith() {
  return (
    [new A()].with(0, new A())[0].run() + [new B()].with(0, new B())[0].run()
  );
}

function useWithIdentVal() {
  const a = new A();
  const b = new B();
  return [new A()].with(0, a)[0].run() + [new B()].with(0, b)[0].run();
}

function useWithLocal() {
  const as = [new A()].with(0, new A());
  const bs = [new B()].with(0, new B());
  return as[0].run() + bs[0].run();
}

function useWithAt() {
  return (
    [new A()].with(0, new A()).at(0).run() +
    [new B()].with(0, new B()).at(0).run()
  );
}

function useWithForOf() {
  let n = 0;
  for (const xa of [new A()].with(0, new A())) {
    n += xa.run();
  }
  for (const xb of [new B()].with(0, new B())) {
    n += xb.run();
  }
  return n;
}

function useWithNegIndex() {
  return (
    [new A()].with(-1, new A())[0].run() + [new B()].with(-1, new B())[0].run()
  );
}

function useArrayFromWith() {
  return (
    Array.from([new A()]).with(0, new A())[0].run() +
    Array.from([new B()]).with(0, new B())[0].run()
  );
}

function usePreservesB() {
  return (
    [new B()].with(0, new B())[0].run() +
    [new B()].with(-1, new B())[0].run()
  );
}
