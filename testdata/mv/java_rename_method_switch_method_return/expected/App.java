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
  // Switch-assign method-return under foreign same-leaf.
  int useSwitchAssignMR(boolean c, BoxA ba, BoxB bb) {
    var mrA = switch (c) {
      case true -> ba.get();
      case false -> ba.get();
    };
    var mrB = switch (c) {
      case true -> bb.get();
      case false -> bb.get();
    };
    return mrA.execute() + mrB.run();
  }

  // Inline already worked.
  int useSwitchInlineMR(boolean c, BoxA ba, BoxB bb) {
    return (switch (c) {
          case true -> ba.get();
          case false -> ba.get();
        })
        .execute()
      + (switch (c) {
            case true -> bb.get();
            case false -> bb.get();
          })
          .run();
  }

  // Class regression — already worked.
  int useSwitchAssignClass(boolean c) {
    var classA = switch (c) {
      case true -> new A();
      case false -> new A();
    };
    var classB = switch (c) {
      case true -> new B();
      case false -> new B();
    };
    return classA.execute() + classB.run();
  }

  int usePreservesB(boolean c, BoxB bb) {
    var mrB = switch (c) {
      case true -> bb.get();
      case false -> bb.get();
    };
    return mrB.run();
  }
}
