package demo;

import java.util.HashMap;
import java.util.Map;

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
  public static int useComputeIfAbsent(Map<String, A> as, Map<String, B> bs) {
    var xa = as.computeIfAbsent("k", k -> new A());
    var xb = bs.computeIfAbsent("k", k -> new B());
    return xa.execute() + xb.run();
  }

  public static int usePutIfAbsent(Map<String, A> as, Map<String, B> bs) {
    var xa = as.putIfAbsent("k", new A());
    var xb = bs.putIfAbsent("k", new B());
    int n = 0;
    if (xa != null) {
      n += xa.execute();
    }
    if (xb != null) {
      n += xb.run();
    }
    return n;
  }

  public static int useHashMap(HashMap<String, A> as, HashMap<String, B> bs) {
    var xa = as.computeIfAbsent("k", k -> new A());
    var xb = bs.computeIfAbsent("k", k -> new B());
    return xa.execute() + xb.run();
  }
}
