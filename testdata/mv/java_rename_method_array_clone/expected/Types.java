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
  public static int useDirect(A[] as, B[] bs) {
    return as.clone()[0].execute() + bs.clone()[0].run();
  }

  public static int useVar(A[] as, B[] bs) {
    var xa = as.clone()[0];
    var xb = bs.clone()[0];
    return xa.execute() + xb.run();
  }

  public static int useVarArray(A[] as, B[] bs) {
    var aa = as.clone();
    var bb = bs.clone();
    return aa[0].execute() + bb[0].run();
  }

  public static int useParen(A[] as, B[] bs) {
    return (as.clone())[0].execute() + (bs.clone())[0].run();
  }

  public static int usePreservesB(B[] bs) {
    var xb = bs.clone()[0];
    return bs.clone()[0].run() + xb.run();
  }
}
