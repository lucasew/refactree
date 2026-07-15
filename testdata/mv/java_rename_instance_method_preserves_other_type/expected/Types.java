package demo;

public class A {
  public int execute() {
    return this.helper();
  }

  public int helper() {
    return 1;
  }
}

class B {
  public int run() {
    return 2;
  }
}

class Uses {
  public static int useA(A a) {
    return a.execute();
  }

  public static int useB(B b) {
    return b.run();
  }
}
