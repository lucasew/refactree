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

class Box {
  A a;
  B b;
}

class Uses {
  public static int useDirect(Box box) {
    return box.a.run() + box.b.run();
  }

  public static int useVar(Box box) {
    var xa = box.a;
    var xb = box.b;
    return xa.run() + xb.run();
  }

  public static int usePreservesB(Box box) {
    return box.b.run();
  }
}
