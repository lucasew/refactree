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
  // forEachValue Consumer applies to V — lambda param under foreign same-leaf.
  public static int useForEachValue(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.forEachValue(1L, a -> a.execute());
    bs.forEachValue(1L, b -> b.run());
    return 0;
  }

  // Block body form.
  public static int useForEachValueBlock(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.forEachValue(1L, a -> {
      a.execute();
    });
    bs.forEachValue(1L, b -> {
      b.run();
    });
    return 0;
  }

  // Regression: searchValues lambda already worked.
  public static int useSearchValues(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
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
}
