package demo;

import java.util.List;

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
  public static int useList(List<A> as, List<B> bs) {
    int n = 0;
    for (var a : as) {
      n += a.run();
    }
    for (var b : bs) {
      n += b.run();
    }
    for (A a2 : as) {
      n += a2.run();
    }
    return n;
  }

  public static int useArray(A[] as, B[] bs) {
    int n = 0;
    for (var a : as) {
      n += a.run();
    }
    for (var b : bs) {
      n += b.run();
    }
    return n;
  }
}
