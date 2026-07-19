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

// Map/WeakMap ctor.get assign method-return under foreign same-leaf.
function useMapGetAssignMR() {
  const mrA = new Map([["k", new BoxA().get()]]).get("k");
  const mrB = new Map([["k", new BoxB().get()]]).get("k");
  return mrA.run() + mrB.run();
}

function useWeakMapGetAssignMR() {
  const ka = {};
  const kb = {};
  const mrWA = new WeakMap([[ka, new BoxA().get()]]).get(ka);
  const mrWB = new WeakMap([[kb, new BoxB().get()]]).get(kb);
  return mrWA.run() + mrWB.run();
}

// Inline / da-assign already worked.
function useMapGetInlineMR() {
  return (
    new Map([["k", new BoxA().get()]]).get("k").run() +
    new Map([["k", new BoxB().get()]]).get("k").run()
  );
}

function useMapGetDaAssignMR() {
  const da = new Map([["k", new BoxA().get()]]);
  const db = new Map([["k", new BoxB().get()]]);
  return da.get("k").run() + db.get("k").run();
}

// Class regression — already worked.
function useMapGetAssignClass() {
  const classA = new Map([["k", new A()]]).get("k");
  const classB = new Map([["k", new B()]]).get("k");
  return classA.run() + classB.run();
}

function useWeakMapGetAssignClass() {
  const ka = {};
  const kb = {};
  const classA = new WeakMap([[ka, new A()]]).get(ka);
  const classB = new WeakMap([[kb, new B()]]).get(kb);
  return classA.run() + classB.run();
}

function usePreservesB() {
  const mrB = new Map([["k", new BoxB().get()]]).get("k");
  const k = {};
  const mrWB = new WeakMap([[k, new BoxB().get()]]).get(k);
  return mrB.run() + mrWB.run();
}
