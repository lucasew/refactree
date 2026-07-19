package demo;

import java.util.List;

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
  public static int useDirect(List<A> as, List<B> bs) {
    return as.stream().toArray()[0].execute() + bs.stream().toArray()[0].run();
  }

  public static int useTyped(List<A> as, List<B> bs) {
    return as.stream().toArray(new A[0])[0].execute() + bs.stream().toArray(new B[0])[0].run();
  }

  public static int useVar(List<A> as, List<B> bs) {
    var xa = as.stream().toArray()[0];
    var xb = bs.stream().toArray()[0];
    return xa.execute() + xb.run();
  }

  public static int useVarTyped(List<A> as, List<B> bs) {
    var xa = as.stream().toArray(new A[0])[0];
    var xb = bs.stream().toArray(new B[0])[0];
    return xa.execute() + xb.run();
  }

  public static int useVarArray(List<A> as, List<B> bs) {
    var aa = as.stream().toArray(new A[0]);
    var bb = bs.stream().toArray(new B[0]);
    return aa[0].execute() + bb[0].run();
  }

  public static int useListToArray(List<A> as, List<B> bs) {
    return as.toArray(new A[0])[0].execute() + bs.toArray(new B[0])[0].run();
  }

  public static int usePreservesB(List<B> bs) {
    var xb = bs.stream().toArray()[0];
    return bs.stream().toArray()[0].run() + xb.run();
  }
}
