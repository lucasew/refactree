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

function useFlatMapIndex() {
  return (
    [new A()].flatMap((x) => [x])[0].run() +
    [new B()].flatMap((x) => [x])[0].run()
  );
}

function useFlatMapBare() {
  return (
    [new A()].flatMap(x => [x])[0].run() + [new B()].flatMap(x => [x])[0].run()
  );
}

function useFlatMapBlock() {
  return (
    [new A()].flatMap((x) => {
      return [x];
    })[0].run() +
    [new B()].flatMap((x) => {
      return [x];
    })[0].run()
  );
}

function useFlatMapFunc() {
  return (
    [new A()].flatMap(function (x) {
      return [x];
    })[0].run() +
    [new B()].flatMap(function (x) {
      return [x];
    })[0].run()
  );
}

function useFlatMapLocal() {
  const as = [new A()].flatMap((x) => [x]);
  const bs = [new B()].flatMap((x) => [x]);
  return as[0].run() + bs[0].run();
}

function useFlatMapAt() {
  return (
    [new A()].flatMap((x) => [x]).at(0).run() +
    [new B()].flatMap((x) => [x]).at(0).run()
  );
}

function useFlatMapForEach() {
  let n = 0;
  [new A()].flatMap((x) => [x]).forEach((va) => {
    n += va.run();
  });
  [new B()].flatMap((x) => [x]).forEach((vb) => {
    n += vb.run();
  });
  return n;
}

function useFlatMapSlice() {
  return (
    [new A()].slice().flatMap((x) => [x])[0].run() +
    [new B()].slice().flatMap((x) => [x])[0].run()
  );
}

function useArrayFromFlatMap() {
  return (
    Array.from([new A()]).flatMap((x) => [x])[0].run() +
    Array.from([new B()]).flatMap((x) => [x])[0].run()
  );
}

function useIdent() {
  const a = new A();
  const b = new B();
  return [a].flatMap((x) => [x])[0].run() + [b].flatMap((x) => [x])[0].run();
}

function usePreservesB() {
  return (
    [new B()].flatMap((x) => [x])[0].run() +
    [new B()].flatMap(x => [x])[0].run() +
    Array.from([new B()]).flatMap((x) => [x])[0].run()
  );
}
