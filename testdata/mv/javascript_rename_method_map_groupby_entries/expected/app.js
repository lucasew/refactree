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

function useEntriesInline() {
  return (
    [...Map.groupBy([new A()], (x) => "k").entries()][0][1][0].execute() +
    [...Map.groupBy([new B()], (x) => "k").entries()][0][1][0].run()
  );
}

function useEntriesLocal() {
  const ma = Map.groupBy([new A()], (x) => "k");
  const mb = Map.groupBy([new B()], (x) => "k");
  return [...ma.entries()][0][1][0].execute() + [...mb.entries()][0][1][0].run();
}

function useEntriesNoSpread() {
  const ma = Map.groupBy([new A()], (x) => "k");
  const mb = Map.groupBy([new B()], (x) => "k");
  // Array.from not required — direct next path covered separately; use spread only
  return [...ma.entries()][0][1][0].execute() + [...mb.entries()][0][1][0].run();
}

function useGroupLocal() {
  const ma = Map.groupBy([new A()], (x) => "k");
  const mb = Map.groupBy([new B()], (x) => "k");
  const xa = [...ma.entries()][0][1];
  const xb = [...mb.entries()][0][1];
  return xa[0].execute() + xb[0].run();
}

function useForOfGroup() {
  let n = 0;
  const ma = Map.groupBy([new A()], (x) => "k");
  const mb = Map.groupBy([new B()], (x) => "k");
  for (const xa of [...ma.entries()][0][1]) {
    n += xa.execute();
  }
  for (const xb of [...mb.entries()][0][1]) {
    n += xb.run();
  }
  return n;
}

function useIdent() {
  const a0 = new A();
  const b0 = new B();
  const ma = Map.groupBy([a0], (x) => "k");
  const mb = Map.groupBy([b0], (x) => "k");
  return [...ma.entries()][0][1][0].execute() + [...mb.entries()][0][1][0].run();
}

function usePreservesB() {
  const mb = Map.groupBy([new B()], (x) => "k");
  return [...mb.entries()][0][1][0].run();
}
