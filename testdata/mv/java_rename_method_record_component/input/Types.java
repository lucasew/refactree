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

record BoxA(A a) {}
record BoxB(B b) {}

class Uses {
  public static int useDirect(BoxA ba, BoxB bb) {
    return ba.a().run() + bb.b().run();
  }

  public static int useVar(BoxA ba, BoxB bb) {
    var xa = ba.a();
    var xb = bb.b();
    return xa.run() + xb.run();
  }

  public static int usePreservesB(BoxB bb) {
    return bb.b().run();
  }
}
