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

// Object-spread destructure method-return under foreign same-leaf.
function useSpreadDestrMR() {
  const { k: mrA } = { ...{ k: new BoxA().get() } };
  const { k: mrB } = { ...{ k: new BoxB().get() } };
  return mrA.execute() + mrB.run();
}

function useSpreadDestrMRMulti() {
  const { k: mrAk, m: mrAm } = { ...{ k: new BoxA().get(), m: new BoxA().get() } };
  const { k: mrBk, m: mrBm } = { ...{ k: new BoxB().get(), m: new BoxB().get() } };
  return mrAk.execute() + mrAm.execute() + mrBk.run() + mrBm.run();
}

function useSpreadChainMR() {
  const xa = { ...{ k: new BoxA().get() } }.k;
  const xb = { ...{ k: new BoxB().get() } }.k;
  return xa.execute() + xb.run();
}

// Class regression — already worked.
function useSpreadDestrClass() {
  const { k: classA } = { ...{ k: new A() } };
  const { k: classB } = { ...{ k: new B() } };
  return classA.execute() + classB.run();
}

function usePreservesB() {
  const { k: mrB } = { ...{ k: new BoxB().get() } };
  return mrB.run() + { ...{ k: new BoxB().get() } }.k.run();
}
