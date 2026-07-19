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

function usePushMR() {
  const xs = [];
  const ys = [];
  xs.push(new BoxA().get());
  ys.push(new BoxB().get());
  return xs[0].run() + ys[0].run();
}

function usePushMRAssign() {
  const xs = [];
  const ys = [];
  xs.push(new BoxA().get());
  ys.push(new BoxB().get());
  const xa = xs[0];
  const xb = ys[0];
  return xa.run() + xb.run();
}

function usePushMRForOf() {
  const xs = [];
  const ys = [];
  xs.push(new BoxA().get());
  ys.push(new BoxB().get());
  let n = 0;
  for (const a of xs) {
    n += a.run();
  }
  for (const b of ys) {
    n += b.run();
  }
  return n;
}

function useUnshiftMR() {
  const xs = [];
  const ys = [];
  xs.unshift(new BoxA().get());
  ys.unshift(new BoxB().get());
  return xs[0].run() + ys[0].run();
}

function usePushMRMulti() {
  const xs = [];
  const ys = [];
  xs.push(new BoxA().get(), new BoxA().get());
  ys.push(new BoxB().get(), new BoxB().get());
  return xs[0].run() + ys[0].run();
}

// Class regression — already worked.
function usePushClass() {
  const xs = [];
  const ys = [];
  xs.push(new A());
  ys.push(new B());
  return xs[0].run() + ys[0].run();
}

function usePreservesB() {
  const ys = [];
  ys.push(new BoxB().get());
  ys.unshift(new BoxB().get());
  return ys[0].run();
}
