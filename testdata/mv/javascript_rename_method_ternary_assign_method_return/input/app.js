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

// Ternary-assign method-return under foreign same-leaf.
function useTernAssignMR(c) {
  const mrA = c ? new BoxA().get() : new BoxA().get();
  const mrB = c ? new BoxB().get() : new BoxB().get();
  return mrA.run() + mrB.run();
}

function useTernAssignLocalMR(c) {
  const ba = new BoxA();
  const bb = new BoxB();
  const mrLA = c ? ba.get() : ba.get();
  const mrLB = c ? bb.get() : bb.get();
  return mrLA.run() + mrLB.run();
}

// Inline already worked via dual-arm shouldRenameMember.
function useTernInlineMR(c) {
  return (
    (c ? new BoxA().get() : new BoxA().get()).run() +
    (c ? new BoxB().get() : new BoxB().get()).run()
  );
}

// Class regression — already worked.
function useTernAssignClass(c) {
  const classA = c ? new A() : new A();
  const classB = c ? new B() : new B();
  return classA.run() + classB.run();
}

function usePreservesB(c) {
  const mrB = c ? new BoxB().get() : new BoxB().get();
  const bb = new BoxB();
  const mrLB = c ? bb.get() : bb.get();
  return mrB.run() + mrLB.run();
}
