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
  // reduceValues BiFunction applies to V,V — both lambda params under foreign same-leaf.
  public static int useReduceValuesBiLambda(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.reduceValues(1L, (a1, a2) -> {
      a1.run();
      a2.run();
      return a1;
    });
    bs.reduceValues(1L, (b1, b2) -> {
      b1.run();
      b2.run();
      return b1;
    });
    return 0;
  }

  // BiFunction return V — chained call.
  public static int useReduceValuesChain(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    return as.reduceValues(1L, (a1, a2) -> a1).run()
        + bs.reduceValues(1L, (b1, b2) -> b1).run();
  }

  // BiFunction return V — var bind.
  public static int useReduceValuesVar(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    var xa = as.reduceValues(1L, (a1, a2) -> a1);
    var xb = bs.reduceValues(1L, (b1, b2) -> b1);
    return xa.run() + xb.run();
  }

  // Regression: searchValues lambda already worked.
  public static int useSearchValues(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.searchValues(1L, a -> {
      a.run();
      return null;
    });
    bs.searchValues(1L, b -> {
      b.run();
      return null;
    });
    return 0;
  }
}
