package demo;

public class A {
  public int run() {
    return 1;
  }
}

class B {
  public int run() {
    return 2;
  }
}

record Box(A inner) {}

class Uses {
  public static int use(Object x) {
    return switch (x) {
      case Box(A a) -> a.run();
      default -> {
        B b = new B();
        yield b.run();
      }
    };
  }

  public static int useInstanceof(Object x) {
    if (x instanceof Box(A a)) {
      return a.run();
    }
    B b = new B();
    return b.run();
  }
}
