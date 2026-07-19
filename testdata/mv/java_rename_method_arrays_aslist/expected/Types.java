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
  public static int useAsListArrayGet(A[] as, B[] bs) {
    return Arrays.asList(as).get(0).execute() + Arrays.asList(bs).get(0).run();
  }

  public static int useAsListArrayForEach(A[] as, B[] bs) {
    Arrays.asList(as).forEach(a -> a.execute());
    Arrays.asList(bs).forEach(b -> b.run());
    return 0;
  }

  public static int useAsListArrayVar(A[] as, B[] bs) {
    var al = Arrays.asList(as);
    var bl = Arrays.asList(bs);
    return al.get(0).execute() + bl.get(0).run();
  }

  public static int useAsListArrayFor(A[] as, B[] bs) {
    int n = 0;
    for (var a : Arrays.asList(as)) {
      n += a.execute();
    }
    for (var b : Arrays.asList(bs)) {
      n += b.run();
    }
    return n;
  }

  public static int useAsListNewArray(A[] as, B[] bs) {
    return Arrays.asList(new A[] { as[0] }).get(0).execute()
        + Arrays.asList(new B[] { bs[0] }).get(0).run();
  }

  public static int useAsListCreationStill() {
    Arrays.asList(new A()).forEach(a -> a.execute());
    Arrays.asList(new B()).forEach(b -> b.run());
    return 0;
  }

  public static int usePreservesB(B[] bs) {
    var xb = Arrays.asList(bs).get(0);
    return Arrays.asList(bs).get(0).run() + xb.run();
  }
}
