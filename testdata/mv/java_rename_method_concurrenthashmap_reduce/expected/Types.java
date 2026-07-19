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
  // reduce transformer BiFunction applies to K,V — value lambda param under foreign same-leaf.
  public static int useReduceBiLambda(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.reduce(1L, (k, va) -> {
      va.execute();
      return va;
    }, (a1, a2) -> a1);
    bs.reduce(1L, (k, vb) -> {
      vb.run();
      return vb;
    }, (b1, b2) -> b1);
    return 0;
  }

  // Identity transformer return U=V — chained call ((k,v) -> v).
  public static int useReduceIdentityChain(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    return as.reduce(1L, (k, va) -> va, (a1, a2) -> a1).execute()
        + bs.reduce(1L, (k, vb) -> vb, (b1, b2) -> b1).run();
  }

  // Identity transformer return U=V — var bind.
  public static int useReduceIdentityVar(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    var xa = as.reduce(1L, (k, va) -> va, (a1, a2) -> a1);
    var xb = bs.reduce(1L, (k, vb) -> vb, (b1, b2) -> b1);
    return xa.execute() + xb.run();
  }

  // Regression: search already worked.
  public static int useSearch(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.search(1L, (k, va) -> {
      va.execute();
      return null;
    });
    bs.search(1L, (k, vb) -> {
      vb.run();
      return null;
    });
    return 0;
  }
}
