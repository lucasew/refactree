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

function useWithAssign() {
  let xs = [];
  let ys = [];
  xs = xs.with(0, new A());
  ys = ys.with(0, new B());
  return xs[0].execute() + ys[0].run();
}

function useWithAssignVar() {
  let xs = [];
  let ys = [];
  xs = xs.with(0, new A());
  ys = ys.with(0, new B());
  const a = xs[0];
  const b = ys[0];
  return a.execute() + b.run();
}

function useWithAssignForOf() {
  let xs = [];
  let ys = [];
  xs = xs.with(0, new A());
  ys = ys.with(0, new B());
  let n = 0;
  for (const a of xs) {
    n += a.execute();
  }
  for (const b of ys) {
    n += b.run();
  }
  return n;
}

function useWithFromEmpty() {
  const xs = [].with(0, new A());
  const ys = [].with(0, new B());
  return xs[0].execute() + ys[0].run();
}

function usePreservesB() {
  let ys = [];
  ys = ys.with(0, new B());
  return ys[0].run();
}
