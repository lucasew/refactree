package demo;

import java.util.Map;
import java.util.NavigableMap;
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
  // Chain: Map.keySet is element-type-preserving for map keys
  // (Set of K; Map stores K in elemOf — same key path as Collections.newSetFromMap).
  public static int useKeySetIterator(Map<A, Boolean> as, Map<B, Boolean> bs) {
    return as.keySet().iterator().next().run()
        + bs.keySet().iterator().next().run();
  }

  public static int useKeySetStream(Map<A, Boolean> as, Map<B, Boolean> bs) {
    return as.keySet().stream().findFirst().get().run()
        + bs.keySet().stream().findFirst().get().run();
  }

  // var from keySet then iterator — element leaf through elemOf.
  public static int useVarKeySet(Map<A, Boolean> as, Map<B, Boolean> bs) {
    var al = as.keySet();
    var bl = bs.keySet();
    var xa = al.iterator().next();
    var xb = bl.iterator().next();
    return xa.run() + xb.run();
  }

  // forEach / for-in through keySet (neighbor paths).
  public static int useKeySetForEach(Map<A, Boolean> as, Map<B, Boolean> bs) {
    as.keySet().forEach(a -> a.run());
    bs.keySet().forEach(b -> b.run());
    return 0;
  }

  public static int useKeySetFor(Map<A, Boolean> as, Map<B, Boolean> bs) {
    int n = 0;
    for (var a : as.keySet()) {
      n += a.run();
    }
    for (var b : bs.keySet()) {
      n += b.run();
    }
    return n;
  }

  // NavigableMap key views (order only; same K).
  public static int useNavigableKeySet(NavigableMap<A, Boolean> as, NavigableMap<B, Boolean> bs) {
    return as.navigableKeySet().iterator().next().run()
        + bs.navigableKeySet().iterator().next().run();
  }

  public static int useDescendingKeySet(NavigableMap<A, Boolean> as, NavigableMap<B, Boolean> bs) {
    return as.descendingKeySet().iterator().next().run()
        + bs.descendingKeySet().iterator().next().run();
  }

  public static int useNavigableKeySetForEach(
      NavigableMap<A, Boolean> as, NavigableMap<B, Boolean> bs) {
    as.navigableKeySet().forEach(a -> a.run());
    bs.descendingKeySet().forEach(b -> b.run());
    return 0;
  }

  // keySet through map view (descendingMap/headMap — same K).
  public static int useViewKeySet(NavigableMap<A, Boolean> as, NavigableMap<B, Boolean> bs) {
    return as.descendingMap().keySet().iterator().next().run()
        + bs.headMap(new B()).keySet().iterator().next().run();
  }

  public static int useTreeMapKeySet(TreeMap<A, Boolean> as, TreeMap<B, Boolean> bs) {
    as.keySet().forEach(a -> a.run());
    bs.keySet().forEach(b -> b.run());
    return 0;
  }
}
