package demo;

import java.util.concurrent.ConcurrentHashMap;

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
  // reduceEntries 3-arg returns U; getValue transformer (e -> e.getValue()) makes U=V.
  // Isolated: no same-name lambda params that file-scoped typedLocals would mask.
  public static int useReduceEntriesTransformChain(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    return as.reduceEntries(1L, ea -> ea.getValue(), (ra1, ra2) -> ra1).run()
        + bs.reduceEntries(1L, eb -> eb.getValue(), (rb1, rb2) -> rb1).run();
  }

  // Var bind of 3-arg reduceEntries U=V return.
  public static int useReduceEntriesTransformVar(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    var xa = as.reduceEntries(1L, ea -> ea.getValue(), (ra1, ra2) -> ra1);
    var xb = bs.reduceEntries(1L, eb -> eb.getValue(), (rb1, rb2) -> rb1);
    return xa.run() + xb.run();
  }

  // Regression: 2-arg reduceEntries returns Entry — chained getValue already worked.
  public static int useReduceEntries2Chain(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    return as.reduceEntries(1L, (ea1, ea2) -> ea1).getValue().run()
        + bs.reduceEntries(1L, (eb1, eb2) -> eb1).getValue().run();
  }
}
