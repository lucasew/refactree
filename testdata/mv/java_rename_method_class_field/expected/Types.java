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
  A a;
  B b;
}

class Uses {
  public static int useDirect(Box box) {
    return box.a.execute() + box.b.run();
  }

  public static int useVar(Box box) {
    var xa = box.a;
    var xb = box.b;
    return xa.execute() + xb.run();
  }

  public static int usePreservesB(Box box) {
    return box.b.run();
  }
}
