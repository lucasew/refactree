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
  // searchValues Function applies to V — lambda param under foreign same-leaf.
  public static int useSearchValuesLambda(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.searchValues(1L, a -> {
      a.execute();
      return null;
    });
    bs.searchValues(1L, b -> {
      b.run();
      return null;
    });
    return 0;
  }

  // Identity Function return U=V — chained call.
  public static int useSearchValuesIdentityChain(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    return as.searchValues(1L, a -> a).execute()
        + bs.searchValues(1L, b -> b).run();
  }

  // Identity Function return U=V — var bind.
  public static int useSearchValuesIdentityVar(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    var xa = as.searchValues(1L, a -> a);
    var xb = bs.searchValues(1L, b -> b);
    return xa.execute() + xb.run();
  }

  // Regression: plain get already worked.
  public static int useGet(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    return as.get("k").execute() + bs.get("k").run();
  }
}
