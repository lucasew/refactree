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
  // reduceValues 3-arg transformer Function applies to V — body under foreign same-leaf.
  public static int useReduceValuesTransform(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.reduceValues(1L, a -> {
      a.execute();
      return a;
    }, (a1, a2) -> a1);
    bs.reduceValues(1L, b -> {
      b.run();
      return b;
    }, (b1, b2) -> b1);
    return 0;
  }

  // Expression body form.
  public static int useReduceValuesTransformExpr(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.reduceValues(1L, a -> { a.execute(); return a; }, (a1, a2) -> a1);
    bs.reduceValues(1L, b -> { b.run(); return b; }, (b1, b2) -> b1);
    return 0;
  }

  // Regression: 2-arg BiFunction reducer already worked.
  public static int useReduceValuesBi(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.reduceValues(1L, (a1, a2) -> {
      a1.execute();
      return a1;
    });
    bs.reduceValues(1L, (b1, b2) -> {
      b1.run();
      return b1;
    });
    return 0;
  }

  // Regression: identity transformer return U=V already worked.
  public static int useReduceValuesIdentityChain(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    return as.reduceValues(1L, a -> a, (a1, a2) -> a1).execute()
        + bs.reduceValues(1L, b -> b, (b1, b2) -> b1).run();
  }
}
