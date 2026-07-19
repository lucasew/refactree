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

function useSpreadAssign() {
  let xs = [];
  let ys = [];
  xs = [...xs, new A()];
  ys = [...ys, new B()];
  return xs[0].run() + ys[0].run();
}

function useSpreadFromEmpty() {
  const xs = [...[], new A()];
  const ys = [...[], new B()];
  return xs[0].run() + ys[0].run();
}

function useSpreadAssignVar() {
  let xs = [];
  let ys = [];
  xs = [...xs, new A()];
  ys = [...ys, new B()];
  const a = xs[0];
  const b = ys[0];
  return a.run() + b.run();
}

function useSpreadAssignForOf() {
  let xs = [];
  let ys = [];
  xs = [...xs, new A()];
  ys = [...ys, new B()];
  let n = 0;
  for (const a of xs) {
    n += a.run();
  }
  for (const b of ys) {
    n += b.run();
  }
  return n;
}

function usePreservesB() {
  let ys = [];
  ys = [...ys, new B()];
  return ys[0].run();
}
