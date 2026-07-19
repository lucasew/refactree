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

function useFlatMapMapLocal() {
  const aa = [[new A()]];
  const bb = [[new B()]];
  return aa.flatMap((xs) => xs.map((x) => x))[0].run()
    + bb.flatMap((xs) => xs.map((x) => x))[0].run();
}

function useFlatMapMapBare() {
  const aa = [[new A()]];
  const bb = [[new B()]];
  return aa.flatMap(xs => xs.map(x => x))[0].run()
    + bb.flatMap(xs => xs.map(x => x))[0].run();
}

function useFlatMapMapBlock() {
  const aa = [[new A()]];
  const bb = [[new B()]];
  return aa.flatMap((xs) => {
    return xs.map((x) => {
      return x;
    });
  })[0].run()
    + bb.flatMap((xs) => {
      return xs.map((x) => {
        return x;
      });
    })[0].run();
}

function useFlatMapMapInline() {
  return [[new A()]].flatMap((xs) => xs.map((x) => x))[0].run()
    + [[new B()]].flatMap((xs) => xs.map((x) => x))[0].run();
}

function usePreservesB() {
  const bb = [[new B()]];
  return bb.flatMap((xs) => xs.map((x) => x))[0].run();
}
