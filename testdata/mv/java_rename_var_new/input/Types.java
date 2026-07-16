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

class Uses {
  public static int use() {
    var a = new A();
    return a.run();
  }

  public static int useB() {
    B b = new B();
    return b.run();
  }
}
