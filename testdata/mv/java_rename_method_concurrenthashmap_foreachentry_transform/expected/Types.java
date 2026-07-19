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
  // forEachEntry 3-arg Consumer applies to U; getValue transformer (e -> e.getValue()) makes U=V.
  // Isolated: no other same-name lambda params (file-scoped typedLocals would mask UNDER).
  public static int useForEachEntryTransform(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.forEachEntry(1L, ea -> ea.getValue(), a -> a.execute());
    bs.forEachEntry(1L, eb -> eb.getValue(), b -> b.run());
    return 0;
  }

  // Block body consumer.
  public static int useForEachEntryTransformBlock(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.forEachEntry(1L, ea -> ea.getValue(), a -> {
      a.execute();
    });
    bs.forEachEntry(1L, eb -> eb.getValue(), b -> {
      b.run();
    });
    return 0;
  }

  // Regression: 2-arg forEachEntry Consumer on Entry already worked.
  // Use ea/eb only — must not bind consumer names a/b used above.
  public static int useForEachEntry2(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.forEachEntry(1L, ea -> ea.getValue().execute());
    bs.forEachEntry(1L, eb -> eb.getValue().run());
    return 0;
  }
}
