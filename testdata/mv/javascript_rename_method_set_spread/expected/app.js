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

function useSpreadLocal() {
  const xs = new Set([new A()]);
  const ys = new Set([new B()]);
  return [...xs][0].execute() + [...ys][0].run();
}

function useArrayFromLocal() {
  const xs = new Set([new A()]);
  const ys = new Set([new B()]);
  return Array.from(xs)[0].execute() + Array.from(ys)[0].run();
}

function useValuesSpread() {
  const xs = new Set([new A()]);
  const ys = new Set([new B()]);
  return [...xs.values()][0].execute() + [...ys.values()][0].run();
}

function useAddSpread() {
  const xs = new Set();
  const ys = new Set();
  xs.add(new A());
  ys.add(new B());
  return [...xs][0].execute() + [...ys][0].run() + [...xs.values()][0].execute() + [...ys.values()][0].run();
}

function useInlineNewSet() {
  return (
    [...new Set([new A()])][0].execute() +
    [...new Set([new B()])][0].run() +
    Array.from(new Set([new A()]))[0].execute() +
    Array.from(new Set([new B()]))[0].run()
  );
}

function usePreservesB() {
  const ys = new Set([new B()]);
  return [...ys][0].run() + [...ys.values()][0].run();
}
