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
  // reduceEntries BiFunction applies to Entry,Entry — getValue under foreign same-leaf.
  public static int useReduceEntriesBiLambda(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.reduceEntries(1L, (ea1, ea2) -> {
      ea1.getValue().run();
      return ea1;
    });
    bs.reduceEntries(1L, (eb1, eb2) -> {
      eb1.getValue().run();
      return eb1;
    });
    return 0;
  }

  // 2-arg reduceEntries returns Entry — chained getValue.
  public static int useReduceEntriesChain(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    return as.reduceEntries(1L, (ea1, ea2) -> ea1).getValue().run()
        + bs.reduceEntries(1L, (eb1, eb2) -> eb1).getValue().run();
  }

  // 2-arg reduceEntries returns Entry — var bind then getValue.
  public static int useReduceEntriesVar(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    var ea = as.reduceEntries(1L, (ea1, ea2) -> ea1);
    var eb = bs.reduceEntries(1L, (eb1, eb2) -> eb1);
    return ea.getValue().run() + eb.getValue().run();
  }

  // searchEntries getValue mapper returns U=V — chain / var.
  public static int useSearchEntriesGetValue(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    var xa = as.searchEntries(1L, e -> e.getValue());
    var xb = bs.searchEntries(1L, e -> e.getValue());
    return as.searchEntries(1L, e -> e.getValue()).run()
        + bs.searchEntries(1L, e -> e.getValue()).run()
        + xa.run()
        + xb.run();
  }

  // Regression: forEachEntry already worked.
  public static int useForEachEntry(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.forEachEntry(1L, ea -> ea.getValue().run());
    bs.forEachEntry(1L, eb -> eb.getValue().run());
    return 0;
  }
}
