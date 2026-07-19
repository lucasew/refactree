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

function useToSplicedAssign() {
  let xs = [];
  let ys = [];
  xs = xs.toSpliced(0, 0, new A());
  ys = ys.toSpliced(0, 0, new B());
  return xs[0].execute() + ys[0].run();
}

function useToSplicedAssignVar() {
  let xs = [];
  let ys = [];
  xs = xs.toSpliced(0, 0, new A());
  ys = ys.toSpliced(0, 0, new B());
  const a = xs[0];
  const b = ys[0];
  return a.execute() + b.run();
}

function useToSplicedAssignForOf() {
  let xs = [];
  let ys = [];
  xs = xs.toSpliced(0, 0, new A());
  ys = ys.toSpliced(0, 0, new B());
  let n = 0;
  for (const a of xs) {
    n += a.execute();
  }
  for (const b of ys) {
    n += b.run();
  }
  return n;
}

function useToSplicedFromEmpty() {
  const xs = [].toSpliced(0, 0, new A());
  const ys = [].toSpliced(0, 0, new B());
  return xs[0].execute() + ys[0].run();
}

function useToSplicedMulti() {
  let xs = [];
  let ys = [];
  xs = xs.toSpliced(0, 0, new A(), new A());
  ys = ys.toSpliced(0, 0, new B(), new B());
  return xs[0].execute() + ys[0].run();
}

function usePreservesB() {
  let ys = [];
  ys = ys.toSpliced(0, 0, new B());
  return ys[0].run();
}
