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

function useToReversed() {
  return (
    [new A()].toReversed()[0].execute() + [new B()].toReversed()[0].run()
  );
}

function useToReversedLocal() {
  const as = [new A()].toReversed();
  const bs = [new B()].toReversed();
  return as[0].execute() + bs[0].run();
}

function useToReversedAt() {
  return (
    [new A()].toReversed().at(0).execute() + [new B()].toReversed().at(0).run()
  );
}

function useToReversedForOf() {
  let n = 0;
  for (const xa of [new A()].toReversed()) {
    n += xa.execute();
  }
  for (const xb of [new B()].toReversed()) {
    n += xb.run();
  }
  return n;
}

function useToSorted() {
  return [new A()].toSorted()[0].execute() + [new B()].toSorted()[0].run();
}

function useToSortedCmp() {
  return (
    [new A()].toSorted((x, y) => 0)[0].execute() +
    [new B()].toSorted((x, y) => 0)[0].run()
  );
}

function useToSortedLocal() {
  const as = [new A()].toSorted();
  const bs = [new B()].toSorted();
  return as[0].execute() + bs[0].run();
}

function useToSortedForOf() {
  let n = 0;
  for (const xa of [new A()].toSorted()) {
    n += xa.execute();
  }
  for (const xb of [new B()].toSorted()) {
    n += xb.run();
  }
  return n;
}

function useArrayFromToReversed() {
  return (
    Array.from([new A()]).toReversed()[0].execute() +
    Array.from([new B()]).toReversed()[0].run()
  );
}

function useIdent() {
  const a = new A();
  const b = new B();
  return [a].toReversed()[0].execute() + [b].toSorted()[0].run();
}

function usePreservesB() {
  return (
    [new B()].toReversed()[0].run() +
    [new B()].toSorted()[0].run() +
    [new B()].toSorted((x, y) => 0)[0].run()
  );
}
