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
    if (x instanceof A a) {
      return a.execute();
    }
    B b = new B();
    return b.run();
  }
}
