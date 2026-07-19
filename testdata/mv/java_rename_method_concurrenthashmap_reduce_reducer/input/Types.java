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
  // reduce 3-arg BiFunction reducer applies to U; value-identity transformer ((k,v) -> v)
  // makes U=V. Isolated: no same-name lambda params that file-scoped typedLocals would mask.
  public static int useReduceIdentityReducer(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.reduce(1L, (k, v) -> v, (a1, a2) -> {
      a1.run();
      return a1;
    });
    bs.reduce(1L, (k, v) -> v, (b1, b2) -> {
      b1.run();
      return b1;
    });
    return 0;
  }

  // Expression-bodied reducer form.
  public static int useReduceIdentityReducerExpr(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.reduce(1L, (k, v) -> v, (c1, c2) -> { c1.run(); return c1; });
    bs.reduce(1L, (k, v) -> v, (d1, d2) -> { d1.run(); return d1; });
    return 0;
  }

  // Regression: value-identity return U=V already worked.
  public static int useReduceIdentityChain(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    return as.reduce(1L, (k, v) -> v, (e1, e2) -> e1).run()
        + bs.reduce(1L, (k, v) -> v, (f1, f2) -> f1).run();
  }
}
