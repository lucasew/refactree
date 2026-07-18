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

function useVaLocal() {
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  const va = Object.values(ga);
  const vb = Object.values(gb);
  return va[0][0].execute() + vb[0][0].run();
}

function useVaLocalElem() {
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  const va = Object.values(ga);
  const vb = Object.values(gb);
  const ga0 = va[0];
  const gb0 = vb[0];
  return ga0[0].execute() + gb0[0].run();
}

function usePreservesB() {
  const gb = Object.groupBy([new B()], (x) => "k");
  const vb = Object.values(gb);
  return vb[0][0].run();
}
