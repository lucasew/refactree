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

function useIndexLiteral() {
  return [new A()][0].execute() + [new B()][0].run();
}

function useIndexLocal() {
  const as = [new A()];
  const bs = [new B()];
  return as[0].execute() + bs[0].run();
}

function useArrayFrom() {
  return (
    Array.from([new A()])[0].execute() + Array.from([new B()])[0].run()
  );
}

function useArrayFromForOf() {
  let n = 0;
  for (const a of Array.from([new A()])) {
    n += a.execute();
  }
  for (const b of Array.from([new B()])) {
    n += b.run();
  }
  return n;
}

function useObjectValues() {
  return (
    Object.values({ k: new A() })[0].execute() +
    Object.values({ k: new B() })[0].run()
  );
}

function useObjectValuesForOf() {
  let n = 0;
  for (const a of Object.values({ k: new A() })) {
    n += a.execute();
  }
  for (const b of Object.values({ k: new B() })) {
    n += b.run();
  }
  return n;
}

function useObjectValuesLocal() {
  const av = Object.values({ k: new A() });
  const bv = Object.values({ k: new B() });
  return av[0].execute() + bv[0].run();
}

function useIdent() {
  const a = new A();
  const b = new B();
  return (
    [a][0].execute() +
    [b][0].run() +
    Array.from([a])[0].execute() +
    Array.from([b])[0].run() +
    Object.values({ k: a })[0].execute() +
    Object.values({ k: b })[0].run()
  );
}

function useShorthand() {
  const a = new A();
  const b = new B();
  return Object.values({ a })[0].execute() + Object.values({ b })[0].run();
}

function usePreservesB() {
  return (
    [new B()][0].run() +
    Array.from([new B()])[0].run() +
    Object.values({ k: new B() })[0].run()
  );
}
