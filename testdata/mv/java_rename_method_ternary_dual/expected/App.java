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

class App {
  int useTyped(A a, A x, B b, B y, boolean c) {
    return (c ? a : x).execute() + (c ? b : y).run();
  }

  int useCtor(boolean c) {
    return (c ? new A() : new A()).execute() + (c ? new B() : new B()).run();
  }

  int useAssign(A a, A x, B b, B y, boolean c) {
    A xa = c ? a : x;
    B xb = c ? b : y;
    return xa.execute() + xb.run();
  }
}
