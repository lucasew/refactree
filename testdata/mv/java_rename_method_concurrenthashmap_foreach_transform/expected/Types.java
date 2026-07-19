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
  // forEach 3-arg Consumer applies to U; value-identity transformer ((k,v)->v) makes U=V.
  // Isolated: no other same-name lambda params (file-scoped typedLocals would mask UNDER).
  public static int useForEachTransform(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.forEach(1L, (k, va) -> va, a -> a.execute());
    bs.forEach(1L, (k, vb) -> vb, b -> b.run());
    return 0;
  }

  // Block body consumer.
  public static int useForEachTransformBlock(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.forEach(1L, (k, va) -> va, a -> {
      a.execute();
    });
    bs.forEach(1L, (k, vb) -> vb, b -> {
      b.run();
    });
    return 0;
  }

  // Regression: 2-arg forEach BiConsumer (value is second param) already worked.
  // Use va/vb only — must not bind consumer names a/b used above.
  public static int useForEachBi(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.forEach(1L, (k, va) -> {
      va.execute();
    });
    bs.forEach(1L, (k, vb) -> {
      vb.run();
    });
    return 0;
  }
}
