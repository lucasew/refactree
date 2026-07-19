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

function useLocal() {
  const aa = [[new A()]];
  const bb = [[new B()]];
  return aa[0][0].run() + bb[0][0].run();
}

function useFlatLocal() {
  const aa = [[new A()]];
  const bb = [[new B()]];
  return aa.flat()[0].run() + bb.flat()[0].run();
}

function useInline() {
  return [[new A()]][0][0].run() + [[new B()]][0][0].run();
}

function usePreservesB() {
  const bb = [[new B()]];
  return bb[0][0].run();
}
