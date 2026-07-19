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

function useFromMapIdentity() {
  return (
    Array.from([new A()], (x) => x)[0].run() +
    Array.from([new B()], (x) => x)[0].run()
  );
}

function useFromMapBare() {
  return (
    Array.from([new A()], (x) => x)[0].run() +
    Array.from([new B()], (x) => x)[0].run()
  );
}

function useFromMapBlock() {
  return (
    Array.from([new A()], (x) => {
      return x;
    })[0].run() +
    Array.from([new B()], (x) => {
      return x;
    })[0].run()
  );
}

function useFromMapForOf() {
  let n = 0;
  for (const xa of Array.from([new A()], (x) => x)) {
    n += xa.run();
  }
  for (const xb of Array.from([new B()], (x) => x)) {
    n += xb.run();
  }
  return n;
}

function useFromMapLocal() {
  const as = Array.from([new A()], (x) => x);
  const bs = Array.from([new B()], (x) => x);
  return as[0].run() + bs[0].run();
}

function useIdent() {
  const a0 = new A();
  const b0 = new B();
  return (
    Array.from([a0], (x) => x)[0].run() + Array.from([b0], (x) => x)[0].run()
  );
}

function usePreservesB() {
  return Array.from([new B()], (x) => x)[0].run();
}
