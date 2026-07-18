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
  public static int useComputeIfPresent(Map<String, A> am, Map<String, B> bm) {
    var xa = am.computeIfPresent("k", (k, v) -> v);
    var xb = bm.computeIfPresent("k", (k, v) -> v);
    return xa.execute() + xb.run();
  }

  public static int useCompute(Map<String, A> am, Map<String, B> bm) {
    var ya = am.compute("k", (k, v) -> v);
    var yb = bm.compute("k", (k, v) -> v);
    return ya.execute() + yb.run();
  }

  public static int useHashMap(HashMap<String, A> am, HashMap<String, B> bm) {
    var za = am.computeIfPresent("k", (k, v) -> v);
    var zb = bm.compute("k", (k, v) -> v);
    return za.execute() + zb.run();
  }
}
