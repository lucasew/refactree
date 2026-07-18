package demo;

import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.TreeMap;

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
  public static int useHashMapOfGet() {
    return new HashMap<>(Map.of("k", new A())).get("k").run()
        + new HashMap<>(Map.of("k", new B())).get("k").run();
  }

  public static int useLinkedHashMapLocal(Map<String, A> am, Map<String, B> bm) {
    return new LinkedHashMap<>(am).get("k").run()
        + new LinkedHashMap<>(bm).get("k").run();
  }

  public static int useTreeMapValuesForEach() {
    new TreeMap<>(Map.of("k", new A())).values().forEach(a -> a.run());
    new TreeMap<>(Map.of("k", new B())).values().forEach(b -> b.run());
    return 0;
  }

  public static int useVarCopy(Map<String, A> am, Map<String, B> bm) {
    var ma = new HashMap<>(am);
    var mb = new HashMap<>(bm);
    return ma.get("k").run() + mb.get("k").run();
  }

  public static int usePreservesB(Map<String, B> bm) {
    return new HashMap<>(bm).get("k").run()
        + new LinkedHashMap<>(Map.of("k", new B())).get("k").run();
  }
}
