package demo;

public class A implements AutoCloseable {
  public int execute() {
    return 1;
  }

  public void close() {}
}

class B {
  public int run() {
    return 2;
  }
}

class Uses {
  public static int use() {
    try (A a = new A()) {
      return a.execute();
    }
  }

  public static int useB() {
    B b = new B();
    return b.run();
  }
}
