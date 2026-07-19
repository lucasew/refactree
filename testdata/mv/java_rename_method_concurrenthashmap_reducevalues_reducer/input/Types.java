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
  // reduceValues 3-arg BiFunction reducer applies to U; identity transformer (a -> a)
  // makes U=V. Isolated: no same-name lambda params that file-scoped typedLocals would mask.
  public static int useReduceValuesIdentityReducer(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.reduceValues(1L, xa -> xa, (a1, a2) -> {
      a1.run();
      return a1;
    });
    bs.reduceValues(1L, xb -> xb, (b1, b2) -> {
      b1.run();
      return b1;
    });
    return 0;
  }

  // Expression-bodied reducer form.
  public static int useReduceValuesIdentityReducerExpr(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.reduceValues(1L, ya -> ya, (c1, c2) -> { c1.run(); return c1; });
    bs.reduceValues(1L, yb -> yb, (d1, d2) -> { d1.run(); return d1; });
    return 0;
  }

  // Regression: identity transformer return U=V already worked.
  public static int useReduceValuesIdentityChain(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    return as.reduceValues(1L, za -> za, (e1, e2) -> e1).run()
        + bs.reduceValues(1L, zb -> zb, (f1, f2) -> f1).run();
  }
}
