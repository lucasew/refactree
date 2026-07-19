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

function useForOfDestructure() {
  let n = 0;
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  for (const [, gaArr] of Object.entries(ga)) {
    n += gaArr[0].execute();
  }
  for (const [, gbArr] of Object.entries(gb)) {
    n += gbArr[0].run();
  }
  return n;
}

function useForOfPair() {
  let n = 0;
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  for (const ea of Object.entries(ga)) {
    n += ea[1][0].execute();
  }
  for (const eb of Object.entries(gb)) {
    n += eb[1][0].run();
  }
  return n;
}

function useForOfInline() {
  let n = 0;
  for (const [, gaArr] of Object.entries(Object.groupBy([new A()], (x) => "k"))) {
    n += gaArr[0].execute();
  }
  for (const [, gbArr] of Object.entries(Object.groupBy([new B()], (x) => "k"))) {
    n += gbArr[0].run();
  }
  return n;
}

function usePairLocal() {
  const ea = Object.entries(Object.groupBy([new A()], (x) => "k"))[0];
  const eb = Object.entries(Object.groupBy([new B()], (x) => "k"))[0];
  return ea[1][0].execute() + eb[1][0].run();
}

function useEntriesArrayLocal() {
  const esa = Object.entries(Object.groupBy([new A()], (x) => "k"));
  const esb = Object.entries(Object.groupBy([new B()], (x) => "k"));
  return esa[0][1][0].execute() + esb[0][1][0].run();
}

function usePreservesB() {
  let n = 0;
  const gb = Object.groupBy([new B()], (x) => "k");
  for (const [, g] of Object.entries(gb)) {
    n += g[0].run();
  }
  for (const e of Object.entries(gb)) {
    n += e[1][0].run();
  }
  return n;
}
