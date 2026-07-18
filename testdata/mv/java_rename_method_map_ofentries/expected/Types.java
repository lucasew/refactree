package demo;

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
  public static int useOfEntriesValues() {
    Map.ofEntries(Map.entry("k", new A())).values().forEach(a -> a.execute());
    Map.ofEntries(Map.entry("k", new B())).values().forEach(b -> b.run());
    return 0;
  }

  public static int useOfEntriesForEach() {
    Map.ofEntries(Map.entry("k", new A())).forEach((k, a) -> a.execute());
    Map.ofEntries(Map.entry("k", new B())).forEach((k, b) -> b.run());
    return 0;
  }

  public static int useVarOfEntriesGet() {
    var am = Map.ofEntries(Map.entry("k", new A()));
    var bm = Map.ofEntries(Map.entry("k", new B()));
    var xa = am.get("k");
    var xb = bm.get("k");
    xa.execute();
    xb.run();
    return 0;
  }

  public static int useVarOfEntriesValues() {
    var am = Map.ofEntries(Map.entry("k", new A()), Map.entry("k2", new A()));
    var bm = Map.ofEntries(Map.entry("k", new B()), Map.entry("k2", new B()));
    am.values().forEach(a -> a.execute());
    bm.values().forEach(b -> b.run());
    int n = 0;
    for (var a : am.values()) {
      n += a.execute();
    }
    for (var b : bm.values()) {
      n += b.run();
    }
    return n;
  }

  public static int useVarOfEntriesForEach() {
    var am = Map.ofEntries(Map.entry("k", new A()));
    var bm = Map.ofEntries(Map.entry("k", new B()));
    am.forEach((k, a) -> a.execute());
    bm.forEach((k, b) -> b.run());
    return 0;
  }
}
