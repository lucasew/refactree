package demo;

public class A {
  public int run() {
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
    return a.run();
  }

  public static int useB(B b) {
    return b.run();
  }
}
