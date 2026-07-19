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

// Index assign method-return under foreign same-leaf.
function useIndexAssignMR() {
  const xsIA = [];
  const ysIA = [];
  xsIA[0] = new BoxA().get();
  ysIA[0] = new BoxB().get();
  return xsIA[0].run() + ysIA[0].run();
}

function useIndexAssignMRAssign() {
  const xsIAA = [];
  const ysIAA = [];
  xsIAA[0] = new BoxA().get();
  ysIAA[0] = new BoxB().get();
  const xa = xsIAA[0];
  const xb = ysIAA[0];
  return xa.run() + xb.run();
}

function useIndexAssignMRForOf() {
  const xsIAF = [];
  const ysIAF = [];
  xsIAF[0] = new BoxA().get();
  ysIAF[0] = new BoxB().get();
  let n = 0;
  for (const a of xsIAF) {
    n += a.run();
  }
  for (const b of ysIAF) {
    n += b.run();
  }
  return n;
}

// Class regression — already worked.
function useIndexAssignClass() {
  const xsIAC = [];
  const ysIAC = [];
  xsIAC[0] = new A();
  ysIAC[0] = new B();
  return xsIAC[0].run() + ysIAC[0].run();
}

function usePreservesB() {
  const ysIAPB = [];
  ysIAPB[0] = new BoxB().get();
  return ysIAPB[0].run();
}
