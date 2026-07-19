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

function useEntriesInline() {
  return (
    Object.entries(Object.groupBy([new A()], (x) => "k"))[0][1][0].run() +
    Object.entries(Object.groupBy([new B()], (x) => "k"))[0][1][0].run()
  );
}

function useEntriesLocal() {
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  return Object.entries(ga)[0][1][0].run() + Object.entries(gb)[0][1][0].run();
}

function useEntriesSpread() {
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  return (
    [...Object.entries(ga)][0][1][0].run() + [...Object.entries(gb)][0][1][0].run()
  );
}

function useGroupLocal() {
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  const xa = Object.entries(ga)[0][1];
  const xb = Object.entries(gb)[0][1];
  return xa[0].run() + xb[0].run();
}

function useForOfGroup() {
  let n = 0;
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  for (const xa of Object.entries(ga)[0][1]) {
    n += xa.run();
  }
  for (const xb of Object.entries(gb)[0][1]) {
    n += xb.run();
  }
  return n;
}

function useIdent() {
  const a0 = new A();
  const b0 = new B();
  const ga = Object.groupBy([a0], (x) => "k");
  const gb = Object.groupBy([b0], (x) => "k");
  return Object.entries(ga)[0][1][0].run() + Object.entries(gb)[0][1][0].run();
}

function usePreservesB() {
  const gb = Object.groupBy([new B()], (x) => "k");
  return Object.entries(gb)[0][1][0].run();
}
