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

function useGroupByLocalKey() {
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  return ga["k"][0].execute() + gb["k"][0].run();
}

function useGroupByLocalDot() {
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  return ga.k[0].execute() + gb.k[0].run();
}

function useGroupByLocalForOf() {
  let n = 0;
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  for (const xa of ga["k"]) {
    n += xa.execute();
  }
  for (const xb of gb["k"]) {
    n += xb.run();
  }
  return n;
}

function useGroupByLocalGroup() {
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  const xa = ga["k"][0];
  const xb = gb["k"][0];
  return xa.execute() + xb.run();
}

function useIdent() {
  const a0 = new A();
  const b0 = new B();
  const ga = Object.groupBy([a0], (x) => "k");
  const gb = Object.groupBy([b0], (x) => "k");
  return ga["k"][0].execute() + gb["k"][0].run();
}

function usePreservesB() {
  const gb = Object.groupBy([new B()], (x) => "k");
  return gb["k"][0].run();
}
