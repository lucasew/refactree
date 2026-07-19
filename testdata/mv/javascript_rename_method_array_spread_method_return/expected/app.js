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

// Spread rebind method-return under foreign same-leaf.
function useSpreadMR() {
  let xsSM = [];
  let ysSM = [];
  xsSM = [...xsSM, new BoxA().get()];
  ysSM = [...ysSM, new BoxB().get()];
  return xsSM[0].execute() + ysSM[0].run();
}

function useSpreadMRAssign() {
  let xsSMA = [];
  let ysSMA = [];
  xsSMA = [...xsSMA, new BoxA().get()];
  ysSMA = [...ysSMA, new BoxB().get()];
  const xa = xsSMA[0];
  const xb = ysSMA[0];
  return xa.execute() + xb.run();
}

function useSpreadMRInline() {
  return [...[], new BoxA().get()][0].execute() + [...[], new BoxB().get()][0].run();
}

function useSpreadMRMulti() {
  let xsSMM = [];
  let ysSMM = [];
  xsSMM = [...xsSMM, new BoxA().get(), new BoxA().get()];
  ysSMM = [...ysSMM, new BoxB().get(), new BoxB().get()];
  return xsSMM[0].execute() + ysSMM[0].run();
}

// Class regression — already worked.
function useSpreadClass() {
  let xsSC = [];
  let ysSC = [];
  xsSC = [...xsSC, new A()];
  ysSC = [...ysSC, new B()];
  return xsSC[0].execute() + ysSC[0].run();
}

function usePreservesB() {
  let ysSPB = [];
  ysSPB = [...ysSPB, new BoxB().get()];
  return ysSPB[0].run();
}
