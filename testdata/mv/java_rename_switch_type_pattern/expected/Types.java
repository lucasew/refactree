package demo;

public class A {
  public int execute() {
    return 1;
  }
}

class B {
  public int run() {
    return 2;
  }
}

class Uses {
  public static int use(Object x) {
    return switch (x) {
      case A a -> a.execute();
      case B b -> b.run();
      default -> 0;
    };
  }
}
