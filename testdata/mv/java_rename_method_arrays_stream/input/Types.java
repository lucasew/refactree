package demo;

import java.util.Arrays;

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
  public static int useStreamArray(A[] as, B[] bs) {
    Arrays.stream(as).forEach(a -> a.run());
    Arrays.stream(bs).forEach(b -> b.run());
    return 0;
  }

  public static int useStreamNew() {
    Arrays.stream(new A[] {new A()}).forEach(a -> a.run());
    Arrays.stream(new B[] {new B()}).forEach(b -> b.run());
    return 0;
  }

  public static int useStreamMap(A[] as, B[] bs) {
    int x = Arrays.stream(as).map(a -> a.run()).mapToInt(i -> i).sum();
    int y = Arrays.stream(bs).map(b -> b.run()).mapToInt(i -> i).sum();
    return x + y;
  }

  public static int useStreamRange(A[] as, B[] bs) {
    Arrays.stream(as, 0, 1).forEach(a -> a.run());
    Arrays.stream(bs, 0, 1).forEach(b -> b.run());
    return 0;
  }
}
