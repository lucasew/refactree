class A {
  int run() {
    return 1;
  }
}

class B {
  int run() {
    return 2;
  }
}

class App {
  int useSwitchLocal(A a, A x, B b, B y, int c) {
    return (switch (c) {
      case 0 -> a;
      default -> x;
    }).run()
        + (switch (c) {
          case 0 -> b;
          default -> y;
        }).run();
  }

  int useSwitchAssign(A a, A x, B b, B y, int c) {
    A xa = switch (c) {
      case 0 -> a;
      default -> x;
    };
    B xb = switch (c) {
      case 0 -> b;
      default -> y;
    };
    return xa.run() + xb.run();
  }

  int useSwitchVar(A a, A x, B b, B y, int c) {
    var xa = switch (c) {
      case 0 -> a;
      default -> x;
    };
    var xb = switch (c) {
      case 0 -> b;
      default -> y;
    };
    return xa.run() + xb.run();
  }

  int useSwitchCtor(int c) {
    return (switch (c) {
      case 0 -> new A();
      default -> new A();
    }).run()
        + (switch (c) {
          case 0 -> new B();
          default -> new B();
        }).run();
  }
}
