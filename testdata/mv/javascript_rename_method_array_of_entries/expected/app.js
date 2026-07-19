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

function useArrayOfIndex() {
  return Array.of(new A())[0].execute() + Array.of(new B())[0].run();
}

function useArrayOfMulti() {
  return (
    Array.of(new A(), new A())[1].execute() + Array.of(new B(), new B())[1].run()
  );
}

function useArrayOfLocal() {
  const as = Array.of(new A());
  const bs = Array.of(new B());
  return as[0].execute() + bs[0].run();
}

function useArrayOfForOf() {
  let n = 0;
  for (const a of Array.of(new A())) {
    n += a.execute();
  }
  for (const b of Array.of(new B())) {
    n += b.run();
  }
  return n;
}

function useEntriesIndex() {
  return (
    Object.entries({ k: new A() })[0][1].execute() +
    Object.entries({ k: new B() })[0][1].run()
  );
}

function useEntriesForOfDestructure() {
  let n = 0;
  for (const [, a] of Object.entries({ k: new A() })) {
    n += a.execute();
  }
  for (const [, b] of Object.entries({ k: new B() })) {
    n += b.run();
  }
  return n;
}

function useEntriesPairDestructure() {
  const [, a] = Object.entries({ k: new A() })[0];
  const [, b] = Object.entries({ k: new B() })[0];
  return a.execute() + b.run();
}

function useEntriesValueLocal() {
  const a = Object.entries({ k: new A() })[0][1];
  const b = Object.entries({ k: new B() })[0][1];
  return a.execute() + b.run();
}

function useIdent() {
  const a = new A();
  const b = new B();
  return (
    Array.of(a)[0].execute() +
    Array.of(b)[0].run() +
    Object.entries({ k: a })[0][1].execute() +
    Object.entries({ k: b })[0][1].run()
  );
}

function useShorthand() {
  const a = new A();
  const b = new B();
  return (
    Object.entries({ a })[0][1].execute() + Object.entries({ b })[0][1].run()
  );
}

function usePreservesB() {
  return (
    Array.of(new B())[0].run() + Object.entries({ k: new B() })[0][1].run()
  );
}
