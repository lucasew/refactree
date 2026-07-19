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

function useSplice() {
  const xs = [];
  const ys = [];
  xs.splice(0, 0, new A());
  ys.splice(0, 0, new B());
  return xs[0].run() + ys[0].run();
}

function useSpliceVar() {
  const xs = [];
  const ys = [];
  xs.splice(0, 0, new A());
  ys.splice(0, 0, new B());
  const a = xs[0];
  const b = ys[0];
  return a.run() + b.run();
}

function useSpliceForOf() {
  const xs = [];
  const ys = [];
  xs.splice(0, 0, new A());
  ys.splice(0, 0, new B());
  let n = 0;
  for (const a of xs) {
    n += a.run();
  }
  for (const b of ys) {
    n += b.run();
  }
  return n;
}

function useSpliceMulti() {
  const xs = [];
  const ys = [];
  xs.splice(0, 0, new A(), new A());
  ys.splice(0, 0, new B(), new B());
  return xs[0].run() + ys[0].run();
}

function useSpliceDelete() {
  const xs = [null];
  const ys = [null];
  xs.splice(0, 1, new A());
  ys.splice(0, 1, new B());
  return xs[0].run() + ys[0].run();
}

function usePreservesB() {
  const ys = [];
  ys.splice(0, 0, new B());
  return ys[0].run();
}
