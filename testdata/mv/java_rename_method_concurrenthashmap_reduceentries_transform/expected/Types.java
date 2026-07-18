package demo;

import java.util.concurrent.ConcurrentHashMap;

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
  // reduceEntries 3-arg returns U; getValue transformer (e -> e.getValue()) makes U=V.
  // Isolated: no same-name lambda params that file-scoped typedLocals would mask.
  public static int useReduceEntriesTransformChain(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    return as.reduceEntries(1L, ea -> ea.getValue(), (ra1, ra2) -> ra1).execute()
        + bs.reduceEntries(1L, eb -> eb.getValue(), (rb1, rb2) -> rb1).run();
  }

  // Var bind of 3-arg reduceEntries U=V return.
  public static int useReduceEntriesTransformVar(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    var xa = as.reduceEntries(1L, ea -> ea.getValue(), (va1, va2) -> va1);
    var xb = bs.reduceEntries(1L, eb -> eb.getValue(), (vb1, vb2) -> vb1);
    return xa.execute() + xb.run();
  }

  // 3-arg BiFunction reducer applies to U; getValue transformer → U=V.
  public static int useReduceEntriesTransformReducer(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.reduceEntries(1L, ea -> ea.getValue(), (a1, a2) -> {
      a1.execute();
      return a1;
    });
    bs.reduceEntries(1L, eb -> eb.getValue(), (b1, b2) -> {
      b1.run();
      return b1;
    });
    return 0;
  }

  // Regression: 2-arg reduceEntries returns Entry — chained getValue already worked.
  public static int useReduceEntries2Chain(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    return as.reduceEntries(1L, (ea1, ea2) -> ea1).getValue().execute()
        + bs.reduceEntries(1L, (eb1, eb2) -> eb1).getValue().run();
  }
}
