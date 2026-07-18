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
  // Chain: Collections.newSetFromMap is element-type-preserving for map keys
  // (Set of K from Map<K,Boolean>; same key path as Map.keySet).
  public static int useNewSetFromMapIterator(Map<A, Boolean> as, Map<B, Boolean> bs) {
    return Collections.newSetFromMap(as).iterator().next().run()
        + Collections.newSetFromMap(bs).iterator().next().run();
  }

  public static int useNewSetFromMapStream(Map<A, Boolean> as, Map<B, Boolean> bs) {
    return Collections.newSetFromMap(as).stream().findFirst().get().run()
        + Collections.newSetFromMap(bs).stream().findFirst().get().run();
  }

  // var from wrapper then iterator — element leaf through elemOf.
  public static int useVarNewSetFromMap(Map<A, Boolean> as, Map<B, Boolean> bs) {
    var al = Collections.newSetFromMap(as);
    var bl = Collections.newSetFromMap(bs);
    var xa = al.iterator().next();
    var xb = bl.iterator().next();
    return xa.run() + xb.run();
  }

  // forEach / for-in through wrapper (neighbor paths).
  public static int useWrapperForEach(Map<A, Boolean> as, Map<B, Boolean> bs) {
    Collections.newSetFromMap(as).forEach(a -> a.run());
    Collections.newSetFromMap(bs).forEach(b -> b.run());
    return 0;
  }

  public static int useWrapperFor(Map<A, Boolean> as, Map<B, Boolean> bs) {
    int n = 0;
    for (var a : Collections.newSetFromMap(as)) {
      n += a.run();
    }
    for (var b : Collections.newSetFromMap(bs)) {
      n += b.run();
    }
    return n;
  }
}
