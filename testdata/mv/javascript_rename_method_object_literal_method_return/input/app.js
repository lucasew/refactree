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

// Object literal method-return under foreign same-leaf.
function useObjectLiteralMR() {
  const oaOL = { k: new BoxA().get() };
  const obOL = { k: new BoxB().get() };
  return oaOL.k.run() + obOL.k.run();
}

function useObjectLiteralMRAssign() {
  const oaOLA = { k: new BoxA().get() };
  const obOLA = { k: new BoxB().get() };
  const xa = oaOLA.k;
  const xb = obOLA.k;
  return xa.run() + xb.run();
}

function useObjectLiteralMRValues() {
  const oaOLV = { k: new BoxA().get() };
  const obOLV = { k: new BoxB().get() };
  return Object.values(oaOLV)[0].run() + Object.values(obOLV)[0].run();
}

function useObjectLiteralMRValuesAssign() {
  const oaOLVA = { k: new BoxA().get() };
  const obOLVA = { k: new BoxB().get() };
  const a = Object.values(oaOLVA)[0];
  const b = Object.values(obOLVA)[0];
  return a.run() + b.run();
}

// Class regression — already worked.
function useObjectLiteralClass() {
  const oaOLC = { k: new A() };
  const obOLC = { k: new B() };
  return oaOLC.k.run() + obOLC.k.run();
}

function usePreservesB() {
  const obOLPB = { k: new BoxB().get() };
  return obOLPB.k.run() + Object.values(obOLPB)[0].run();
}
