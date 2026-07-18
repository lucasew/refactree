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

function useFill() {
  return (
    [new A()].fill(new A())[0].execute() + [new B()].fill(new B())[0].run()
  );
}

function useFillRange() {
  return (
    [new A()].fill(new A(), 0, 1)[0].execute() +
    [new B()].fill(new B(), 0, 1)[0].run()
  );
}

function useFillLocal() {
  const as = [new A()].fill(new A());
  const bs = [new B()].fill(new B());
  return as[0].execute() + bs[0].run();
}

function useFillIdent() {
  const a = new A();
  const b = new B();
  return [new A()].fill(a)[0].execute() + [new B()].fill(b)[0].run();
}

function useCopyWithin() {
  return (
    [new A()].copyWithin(0)[0].execute() + [new B()].copyWithin(0)[0].run()
  );
}

function useCopyWithinArgs() {
  return (
    [new A()].copyWithin(0, 0)[0].execute() + [new B()].copyWithin(0, 0)[0].run()
  );
}

function useCopyWithinLocal() {
  const as = [new A()].copyWithin(0);
  const bs = [new B()].copyWithin(0);
  return as[0].execute() + bs[0].run();
}

function useCopyWithinForOf() {
  let n = 0;
  for (const xa of [new A()].copyWithin(0)) {
    n += xa.execute();
  }
  for (const xb of [new B()].copyWithin(0)) {
    n += xb.run();
  }
  return n;
}

function usePreservesB() {
  return (
    [new B()].fill(new B())[0].run() + [new B()].copyWithin(0)[0].run()
  );
}
