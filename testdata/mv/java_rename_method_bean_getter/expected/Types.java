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

class Box {
  private A a;
  private B b;

  public A getA() {
    return a;
  }

  public B getB() {
    return b;
  }
}

class Uses {
  public static int useDirect(Box box) {
    return box.getA().execute() + box.getB().run();
  }

  public static int useVar(Box box) {
    var xa = box.getA();
    var xb = box.getB();
    return xa.execute() + xb.run();
  }

  public static int usePreservesB(Box box) {
    return box.getB().run();
  }
}
