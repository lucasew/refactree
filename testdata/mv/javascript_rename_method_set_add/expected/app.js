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

function useAddForOf() {
  const xs = new Set();
  const ys = new Set();
  xs.add(new A());
  ys.add(new B());
  let n = 0;
  for (const a of xs) {
    n += a.execute();
  }
  for (const b of ys) {
    n += b.run();
  }
  return n;
}

function useAddValues() {
  const xs = new Set();
  const ys = new Set();
  xs.add(new A());
  ys.add(new B());
  return xs.values().next().value.execute() + ys.values().next().value.run();
}

function useAddValuesVar() {
  const xs = new Set();
  const ys = new Set();
  xs.add(new A());
  ys.add(new B());
  const a = xs.values().next().value;
  const b = ys.values().next().value;
  return a.execute() + b.run();
}

function usePreservesB() {
  const ys = new Set();
  ys.add(new B());
  return ys.values().next().value.run();
}
