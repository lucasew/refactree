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

function usePush() {
  const xs = [];
  const ys = [];
  xs.push(new A());
  ys.push(new B());
  return xs[0].run() + ys[0].run();
}

function usePushVar() {
  const xs = [];
  const ys = [];
  xs.push(new A());
  ys.push(new B());
  const a = xs[0];
  const b = ys[0];
  return a.run() + b.run();
}

function usePushForOf() {
  const xs = [];
  const ys = [];
  xs.push(new A());
  ys.push(new B());
  let n = 0;
  for (const a of xs) {
    n += a.run();
  }
  for (const b of ys) {
    n += b.run();
  }
  return n;
}

function useUnshift() {
  const xs = [];
  const ys = [];
  xs.unshift(new A());
  ys.unshift(new B());
  return xs[0].run() + ys[0].run();
}

function usePushMulti() {
  const xs = [];
  const ys = [];
  xs.push(new A(), new A());
  ys.push(new B(), new B());
  return xs[0].run() + ys[0].run();
}

function usePreservesB() {
  const ys = [];
  ys.push(new B());
  return ys[0].run();
}
