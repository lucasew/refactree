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

function usePushSpread() {
  const xs = [];
  const ys = [];
  xs.push(...[new A()]);
  ys.push(...[new B()]);
  return xs[0].execute() + ys[0].run();
}

function usePushSpreadVar() {
  const xs = [];
  const ys = [];
  xs.push(...[new A()]);
  ys.push(...[new B()]);
  const a = xs[0];
  const b = ys[0];
  return a.execute() + b.run();
}

function usePushSpreadForOf() {
  const xs = [];
  const ys = [];
  xs.push(...[new A()]);
  ys.push(...[new B()]);
  let n = 0;
  for (const a of xs) {
    n += a.execute();
  }
  for (const b of ys) {
    n += b.run();
  }
  return n;
}

function usePushSpreadLocal() {
  const as = [new A()];
  const bs = [new B()];
  const xs = [];
  const ys = [];
  xs.push(...as);
  ys.push(...bs);
  return xs[0].execute() + ys[0].run();
}

function usePushSpreadMulti() {
  const xs = [];
  const ys = [];
  xs.push(...[new A(), new A()]);
  ys.push(...[new B(), new B()]);
  return xs[0].execute() + ys[0].run();
}

function usePushMixedSpread() {
  const xs = [];
  const ys = [];
  xs.push(new A(), ...[new A()]);
  ys.push(new B(), ...[new B()]);
  return xs[0].execute() + ys[0].run();
}

function usePreservesB() {
  const ys = [];
  ys.push(...[new B()]);
  return ys[0].run();
}
