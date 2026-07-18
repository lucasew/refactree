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

function useConcatAssign() {
  let xs = [];
  let ys = [];
  xs = xs.concat([new A()]);
  ys = ys.concat([new B()]);
  return xs[0].execute() + ys[0].run();
}

function useConcatAssignElem() {
  let xs = [];
  let ys = [];
  xs = xs.concat(new A());
  ys = ys.concat(new B());
  return xs[0].execute() + ys[0].run();
}

function useConcatFromEmpty() {
  const xs = [].concat([new A()]);
  const ys = [].concat([new B()]);
  return xs[0].execute() + ys[0].run();
}

function useConcatChain() {
  const xs = [].concat(new A());
  const ys = [].concat(new B());
  return xs[0].execute() + ys[0].run();
}

function useConcatAssignForOf() {
  let xs = [];
  let ys = [];
  xs = xs.concat([new A()]);
  ys = ys.concat([new B()]);
  let n = 0;
  for (const a of xs) {
    n += a.execute();
  }
  for (const b of ys) {
    n += b.run();
  }
  return n;
}

function usePreservesB() {
  let ys = [];
  ys = ys.concat([new B()]);
  return ys[0].run();
}
