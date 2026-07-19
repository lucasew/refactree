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

function useIndexAssign() {
  const xs = [];
  const ys = [];
  xs[0] = new A();
  ys[0] = new B();
  return xs[0].execute() + ys[0].run();
}

function useIndexAssignVar() {
  const xs = [];
  const ys = [];
  xs[0] = new A();
  ys[0] = new B();
  const a = xs[0];
  const b = ys[0];
  return a.execute() + b.run();
}

function useIndexAssignForOf() {
  const xs = [];
  const ys = [];
  xs[0] = new A();
  ys[0] = new B();
  let n = 0;
  for (const a of xs) {
    n += a.execute();
  }
  for (const b of ys) {
    n += b.run();
  }
  return n;
}

function useIndexAssignIdentVal() {
  const xs = [];
  const ys = [];
  const a0 = new A();
  const b0 = new B();
  xs[0] = a0;
  ys[0] = b0;
  return xs[0].execute() + ys[0].run();
}

function useIndexAssignNeg() {
  const xs = [null];
  const ys = [null];
  xs[-1] = new A();
  ys[-1] = new B();
  return xs[0].execute() + ys[0].run();
}

function usePreservesB() {
  const ys = [];
  ys[0] = new B();
  return ys[0].run();
}
