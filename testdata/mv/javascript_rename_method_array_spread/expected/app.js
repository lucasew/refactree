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

function useSpread() {
  return [...[new A()]][0].execute() + [...[new B()]][0].run();
}

function useSpreadLocal() {
  const as = [new A()];
  const bs = [new B()];
  return [...as][0].execute() + [...bs][0].run();
}

function useSpreadAssign() {
  const as = [...[new A()]];
  const bs = [...[new B()]];
  return as[0].execute() + bs[0].run();
}

function useSpreadAt() {
  return [...[new A()]].at(0).execute() + [...[new B()]].at(0).run();
}

function useSpreadForOf() {
  let n = 0;
  for (const xa of [...[new A()]]) {
    n += xa.execute();
  }
  for (const xb of [...[new B()]]) {
    n += xb.run();
  }
  return n;
}

function useSpreadForOfLocal() {
  const as = [new A()];
  const bs = [new B()];
  let n = 0;
  for (const xa of [...as]) {
    n += xa.execute();
  }
  for (const xb of [...bs]) {
    n += xb.run();
  }
  return n;
}

function useSpreadExtra() {
  return (
    [...[new A()], new A()][0].execute() + [...[new B()], new B()][0].run()
  );
}

function useSpreadArrayFrom() {
  return (
    [...Array.from([new A()])][0].execute() + [...Array.from([new B()])][0].run()
  );
}

function useIdent() {
  const a = new A();
  const b = new B();
  return [...[a]][0].execute() + [...[b]][0].run();
}

function usePreservesB() {
  return (
    [...[new B()]][0].run() +
    [...[new B()], new B()][0].run() +
    [...Array.from([new B()])][0].run()
  );
}
