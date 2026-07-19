package demo;

import java.util.Collections;
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
  public static int useUnmodifiableMapValues(Map<String, A> as, Map<String, B> bs) {
    Collections.unmodifiableMap(as).values().forEach(a -> a.run());
    Collections.unmodifiableMap(bs).values().forEach(b -> b.run());
    return 0;
  }

  public static int useSynchronizedMapForEach(Map<String, A> as, Map<String, B> bs) {
    Collections.synchronizedMap(as).forEach((k, a) -> a.run());
    Collections.synchronizedMap(bs).forEach((k, b) -> b.run());
    return 0;
  }

  public static int useCheckedMapGet(Map<String, A> as, Map<String, B> bs) {
    var am = Collections.checkedMap(as, String.class, A.class);
    var bm = Collections.checkedMap(bs, String.class, B.class);
    var xa = am.get("k");
    var xb = bm.get("k");
    xa.run();
    xb.run();
    return 0;
  }

  public static int useSingletonMap() {
    Collections.singletonMap("k", new A()).values().forEach(a -> a.run());
    Collections.singletonMap("k", new B()).values().forEach(b -> b.run());
    return 0;
  }

  public static int useVarSingletonMap() {
    var am = Collections.singletonMap("k", new A());
    var bm = Collections.singletonMap("k", new B());
    am.values().forEach(a -> a.run());
    bm.values().forEach(b -> b.run());
    am.forEach((k, a) -> a.run());
    bm.forEach((k, b) -> b.run());
    return 0;
  }

  public static int useMapOf() {
    Map.of("k", new A()).values().forEach(a -> a.run());
    Map.of("k", new B()).values().forEach(b -> b.run());
    return 0;
  }

  public static int useVarMapOf() {
    var am = Map.of("k", new A());
    var bm = Map.of("k", new B());
    var xa = am.get("k");
    var xb = bm.get("k");
    xa.run();
    xb.run();
    return 0;
  }

  public static int useMapCopyOf(Map<String, A> as, Map<String, B> bs) {
    Map.copyOf(as).values().forEach(a -> a.run());
    Map.copyOf(bs).values().forEach(b -> b.run());
    return 0;
  }

  public static int useVarMapWrappers(Map<String, A> as, Map<String, B> bs) {
    var am = Collections.unmodifiableMap(as);
    var bm = Collections.synchronizedMap(bs);
    am.values().forEach(a -> a.run());
    bm.values().forEach(b -> b.run());
    int n = 0;
    for (var a : am.values()) {
      n += a.run();
    }
    for (var b : bm.values()) {
      n += b.run();
    }
    return n;
  }
}
