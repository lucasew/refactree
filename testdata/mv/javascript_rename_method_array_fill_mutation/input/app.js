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

function useFillMutation() {
  const xs = [null];
  const ys = [null];
  xs.fill(new A());
  ys.fill(new B());
  return xs[0].run() + ys[0].run();
}

function useFillMutationVar() {
  const xs = [null];
  const ys = [null];
  xs.fill(new A());
  ys.fill(new B());
  const a = xs[0];
  const b = ys[0];
  return a.run() + b.run();
}

function useFillMutationForOf() {
  const xs = [null];
  const ys = [null];
  xs.fill(new A());
  ys.fill(new B());
  let n = 0;
  for (const a of xs) {
    n += a.run();
  }
  for (const b of ys) {
    n += b.run();
  }
  return n;
}

function useFillRange() {
  const xs = [null, null];
  const ys = [null, null];
  xs.fill(new A(), 0, 2);
  ys.fill(new B(), 0, 2);
  return xs[0].run() + ys[0].run();
}

function useFillExpr() {
  return (
    [null].fill(new A())[0].run() + [null].fill(new B())[0].run()
  );
}

function useFillExprLocal() {
  const as = [null].fill(new A());
  const bs = [null].fill(new B());
  return as[0].run() + bs[0].run();
}

function useFillIdent() {
  const a = new A();
  const b = new B();
  const xs = [null];
  const ys = [null];
  xs.fill(a);
  ys.fill(b);
  return xs[0].run() + ys[0].run();
}

function usePreservesB() {
  const ys = [null];
  ys.fill(new B());
  return ys[0].run();
}
