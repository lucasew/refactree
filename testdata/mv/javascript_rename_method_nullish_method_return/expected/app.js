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

// Nullish/or/and method-return under foreign same-leaf.
function useNullishMR() {
  const aN = null ?? new BoxA().get();
  const bN = null ?? new BoxB().get();
  return aN.execute() + bN.run();
}

function useOrMR() {
  const aO = null || new BoxA().get();
  const bO = null || new BoxB().get();
  return aO.execute() + bO.run();
}

function useAndMR() {
  const aA = true && new BoxA().get();
  const bA = true && new BoxB().get();
  return aA.execute() + bA.run();
}

function useNullishMRParen() {
  const aP = null ?? new BoxA().get();
  const bP = null ?? new BoxB().get();
  return (aP).execute() + (bP).run();
}

// Inline form already worked for MR; keep as regression.
function useNullishInlineMR() {
  return (null ?? new BoxA().get()).execute() + (null ?? new BoxB().get()).run();
}

// Class regression — already worked.
function useNullishClass() {
  const aC = null ?? new A();
  const bC = null ?? new B();
  return aC.execute() + bC.run();
}

function usePreservesB() {
  const bPB = null ?? new BoxB().get();
  return bPB.run();
}
