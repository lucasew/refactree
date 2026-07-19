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

function useMapThenFlatLocal() {
  const aa = [[new A()]];
  const bb = [[new B()]];
  return aa.map((xs) => xs).flat()[0].execute()
    + bb.map((xs) => xs).flat()[0].run();
}

function useMapThenFlatBare() {
  const aa = [[new A()]];
  const bb = [[new B()]];
  return aa.map(xs => xs).flat()[0].execute()
    + bb.map(xs => xs).flat()[0].run();
}

function useMapThenFlatInline() {
  return [[new A()]].map((xs) => xs).flat()[0].execute()
    + [[new B()]].map((xs) => xs).flat()[0].run();
}

function useMapIdentityThenFlat() {
  const aa = [[new A()]];
  const bb = [[new B()]];
  return aa.map((xs) => xs.map((x) => x)).flat()[0].execute()
    + bb.map((xs) => xs.map((x) => x)).flat()[0].run();
}

function useMapThenFlatDepth1() {
  const aa = [[new A()]];
  const bb = [[new B()]];
  return aa.map((xs) => xs).flat(1)[0].execute()
    + bb.map((xs) => xs).flat(1)[0].run();
}

function useMapLocalThenFlat() {
  const aa = [[new A()]];
  const bb = [[new B()]];
  const ma = aa.map((xs) => xs);
  const mb = bb.map((xs) => xs);
  return ma.flat()[0].execute() + mb.flat()[0].run();
}

function usePreservesB() {
  const bb = [[new B()]];
  return bb.map((xs) => xs).flat()[0].run();
}
