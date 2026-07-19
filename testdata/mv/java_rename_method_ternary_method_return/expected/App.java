class A {
  int execute() {
    return 1;
  }
}

class B {
  int run() {
    return 2;
  }
}

class BoxA {
  A a = new A();

  A get() {
    return a;
  }
}

class BoxB {
  B b = new B();

  B get() {
    return b;
  }
}

class App {
  // Ternary-assign method-return under foreign same-leaf.
  int useTernaryAssignMR(boolean c, BoxA ba, BoxB bb) {
    var mrA = c ? ba.get() : ba.get();
    var mrB = c ? bb.get() : bb.get();
    return mrA.execute() + mrB.run();
  }

  // Inline already worked.
  int useTernaryInlineMR(boolean c, BoxA ba, BoxB bb) {
    return (c ? ba.get() : ba.get()).execute() + (c ? bb.get() : bb.get()).run();
  }

  // Class regression — already worked.
  int useTernaryAssignClass(boolean c) {
    var classA = c ? new A() : new A();
    var classB = c ? new B() : new B();
    return classA.execute() + classB.run();
  }

  int usePreservesB(boolean c, BoxB bb) {
    var mrB = c ? bb.get() : bb.get();
    return mrB.run();
  }
}
