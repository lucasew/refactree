package demo;

import java.util.Arrays;

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
  public static int useCopyOfDirect(A[] as, B[] bs) {
    return Arrays.copyOf(as, as.length)[0].execute() + Arrays.copyOf(bs, bs.length)[0].run();
  }

  public static int useCopyOfRangeDirect(A[] as, B[] bs) {
    return Arrays.copyOfRange(as, 0, 1)[0].execute() + Arrays.copyOfRange(bs, 0, 1)[0].run();
  }

  public static int useCopyOfVar(A[] as, B[] bs) {
    var xa = Arrays.copyOf(as, as.length)[0];
    var xb = Arrays.copyOf(bs, bs.length)[0];
    return xa.execute() + xb.run();
  }

  public static int useCopyOfRangeVar(A[] as, B[] bs) {
    var xa = Arrays.copyOfRange(as, 0, 1)[0];
    var xb = Arrays.copyOfRange(bs, 0, 1)[0];
    return xa.execute() + xb.run();
  }

  public static int useCopyOfVarArray(A[] as, B[] bs) {
    var aa = Arrays.copyOf(as, as.length);
    var bb = Arrays.copyOf(bs, bs.length);
    return aa[0].execute() + bb[0].run();
  }

  public static int useCopyOfRangeVarArray(A[] as, B[] bs) {
    var aa = Arrays.copyOfRange(as, 0, as.length);
    var bb = Arrays.copyOfRange(bs, 0, bs.length);
    return aa[0].execute() + bb[0].run();
  }

  public static int useCopyOfNew(A[] as, B[] bs) {
    return Arrays.copyOf(new A[] { as[0] }, 1)[0].execute()
        + Arrays.copyOf(new B[] { bs[0] }, 1)[0].run();
  }

  public static int usePreservesB(B[] bs) {
    var xb = Arrays.copyOf(bs, bs.length)[0];
    return Arrays.copyOf(bs, bs.length)[0].run() + xb.run();
  }
}
