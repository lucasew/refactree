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

class BoxA {
  a = new A();
  get() {
    return this.a;
  }
}

class BoxB {
  b = new B();
  get() {
    return this.b;
  }
}

function useAddMRForOf() {
  const xs = new Set();
  const ys = new Set();
  xs.add(new BoxA().get());
  ys.add(new BoxB().get());
  let n = 0;
  for (const a of xs) {
    n += a.execute();
  }
  for (const b of ys) {
    n += b.run();
  }
  return n;
}

function useAddMRValues() {
  const xs = new Set();
  const ys = new Set();
  xs.add(new BoxA().get());
  ys.add(new BoxB().get());
  return xs.values().next().value.execute() + ys.values().next().value.run();
}

function useAddMRValuesAssign() {
  const xs = new Set();
  const ys = new Set();
  xs.add(new BoxA().get());
  ys.add(new BoxB().get());
  const xa = xs.values().next().value;
  const xb = ys.values().next().value;
  return xa.execute() + xb.run();
}

// Class regression — already worked.
function useAddClass() {
  const xs = new Set();
  const ys = new Set();
  xs.add(new A());
  ys.add(new B());
  return xs.values().next().value.execute() + ys.values().next().value.run();
}

function usePreservesB() {
  const ys = new Set();
  ys.add(new BoxB().get());
  return ys.values().next().value.run();
}
