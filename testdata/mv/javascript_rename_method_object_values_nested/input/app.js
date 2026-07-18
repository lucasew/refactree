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

function useObjectValuesNested() {
  return Object.values({k: [new A()]})[0][0].run()
    + Object.values({k: [new B()]})[0][0].run();
}

function useObjectValuesNestedLocal() {
  const oa = {k: [new A()]};
  const ob = {k: [new B()]};
  return Object.values(oa)[0][0].run() + Object.values(ob)[0][0].run();
}

function useObjectValuesNestedVa() {
  const oa = {k: [new A()]};
  const ob = {k: [new B()]};
  const va = Object.values(oa);
  const vb = Object.values(ob);
  return va[0][0].run() + vb[0][0].run();
}

function useNestedFlatMapLocal() {
  const aa = [[new A()]];
  const bb = [[new B()]];
  return aa.flatMap((xs) => xs)[0].run() + bb.flatMap((xs) => xs)[0].run();
}

function usePreservesB() {
  return Object.values({k: [new B()]})[0][0].run();
}
