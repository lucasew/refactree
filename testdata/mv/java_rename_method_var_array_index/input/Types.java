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
  public static int useDirect(A[] as, B[] bs) {
    return as[0].run() + bs[0].run();
  }

  public static int useVar(A[] as, B[] bs) {
    var xa = as[0];
    var xb = bs[0];
    return xa.run() + xb.run();
  }

  public static int useVarParen(A[] as, B[] bs) {
    var xa = (as)[0];
    var xb = (bs)[0];
    return xa.run() + xb.run();
  }

  public static int useVarNew(A[] as, B[] bs) {
    var xa = new A[] { as[0] }[0];
    var xb = new B[] { bs[0] }[0];
    return xa.run() + xb.run();
  }

  public static int usePreservesB(B[] bs) {
    var xb = bs[0];
    return bs[0].run() + xb.run();
  }
}
