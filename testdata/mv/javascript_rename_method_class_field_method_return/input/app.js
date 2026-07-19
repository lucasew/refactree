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

// Class field method-return under foreign same-leaf.
class HolderMR {
  mrA = new BoxA().get();
  mrB = new BoxB().get();
}

class HolderClass {
  classA = new A();
  classB = new B();
}

function useFieldMR() {
  const hMR = new HolderMR();
  return hMR.mrA.run() + hMR.mrB.run();
}

function useFieldMRAssign() {
  const hMRA = new HolderMR();
  const xa = hMRA.mrA;
  const xb = hMRA.mrB;
  return xa.run() + xb.run();
}

function useFieldMRNew() {
  return new HolderMR().mrA.run() + new HolderMR().mrB.run();
}

// Class regression — already worked.
function useFieldClass() {
  const hC = new HolderClass();
  return hC.classA.run() + hC.classB.run();
}

function usePreservesB() {
  const hPB = new HolderMR();
  return hPB.mrB.run() + new HolderMR().mrB.run();
}
