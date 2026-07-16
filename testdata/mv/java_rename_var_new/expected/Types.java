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
  public static int use() {
    var a = new A();
    return a.execute();
  }

  public static int useB() {
    B b = new B();
    return b.run();
  }
}
