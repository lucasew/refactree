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
  // search BiFunction applies to K,V — value lambda param under foreign same-leaf.
  public static int useSearchBiLambda(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.search(1L, (k, va) -> {
      va.run();
      return null;
    });
    bs.search(1L, (k, vb) -> {
      vb.run();
      return null;
    });
    return 0;
  }

  // Identity BiFunction return U=V — chained call ((k,v) -> v).
  public static int useSearchIdentityChain(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    return as.search(1L, (k, va) -> va).run()
        + bs.search(1L, (k, vb) -> vb).run();
  }

  // Identity BiFunction return U=V — var bind.
  public static int useSearchIdentityVar(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    var xa = as.search(1L, (k, va) -> va);
    var xb = bs.search(1L, (k, vb) -> vb);
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
