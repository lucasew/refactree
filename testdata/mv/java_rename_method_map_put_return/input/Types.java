package demo;

import java.util.Map;

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
  public static int usePut(Map<String, A> am, Map<String, B> bm) {
    var xa = am.put("k", new A());
    var xb = bm.put("k", new B());
    return xa.run() + xb.run();
  }

  public static int useReplace(Map<String, A> am, Map<String, B> bm) {
    var ya = am.replace("k", new A());
    var yb = bm.replace("k", new B());
    return ya.run() + yb.run();
  }

  public static int useMerge(Map<String, A> am, Map<String, B> bm) {
    var za = am.merge("k", new A(), (a1, a2) -> a1);
    var zb = bm.merge("k", new B(), (b1, b2) -> b1);
    return za.run() + zb.run();
  }
}
