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

function useMapIndex() {
  return [new A()].map((x) => x)[0].run() + [new B()].map((x) => x)[0].run();
}

function useMapBare() {
  return [new A()].map(x => x)[0].run() + [new B()].map(x => x)[0].run();
}

function useMapBlock() {
  return (
    [new A()].map((x) => {
      return x;
    })[0].run() +
    [new B()].map((x) => {
      return x;
    })[0].run()
  );
}

function useMapFunc() {
  return (
    [new A()].map(function (x) {
      return x;
    })[0].run() +
    [new B()].map(function (x) {
      return x;
    })[0].run()
  );
}

function useMapLocal() {
  const as = [new A()].map((x) => x);
  const bs = [new B()].map((x) => x);
  return as[0].run() + bs[0].run();
}

function useMapAt() {
  return (
    [new A()].map((x) => x).at(0).run() + [new B()].map((x) => x).at(0).run()
  );
}

function useMapForEach() {
  let n = 0;
  [new A()].map((x) => x).forEach((va) => {
    n += va.run();
  });
  [new B()].map((x) => x).forEach((vb) => {
    n += vb.run();
  });
  return n;
}

function useMapSlice() {
  return (
    [new A()].slice().map((x) => x)[0].run() +
    [new B()].slice().map((x) => x)[0].run()
  );
}

function useArrayFromMap() {
  return (
    Array.from([new A()]).map((x) => x)[0].run() +
    Array.from([new B()]).map((x) => x)[0].run()
  );
}

function useIdent() {
  const a = new A();
  const b = new B();
  return [a].map((x) => x)[0].run() + [b].map((x) => x)[0].run();
}

function usePreservesB() {
  return (
    [new B()].map((x) => x)[0].run() +
    [new B()].map(x => x)[0].run() +
    Array.from([new B()]).map((x) => x)[0].run()
  );
}
