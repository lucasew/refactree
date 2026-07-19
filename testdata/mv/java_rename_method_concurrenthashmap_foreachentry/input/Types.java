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
  // forEachEntry Consumer applies to Map.Entry — getValue under foreign same-leaf.
  public static int useForEachEntry(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.forEachEntry(1L, ea -> ea.getValue().run());
    bs.forEachEntry(1L, eb -> eb.getValue().run());
    return 0;
  }

  // Block body form.
  public static int useForEachEntryBlock(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.forEachEntry(1L, ea -> {
      ea.getValue().run();
    });
    bs.forEachEntry(1L, eb -> {
      eb.getValue().run();
    });
    return 0;
  }

  // searchEntries Function applies to Map.Entry — getValue under foreign same-leaf.
  public static int useSearchEntries(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.searchEntries(1L, ea -> {
      ea.getValue().run();
      return null;
    });
    bs.searchEntries(1L, eb -> {
      eb.getValue().run();
      return null;
    });
    return 0;
  }

  // Regression: forEachValue lambda already worked.
  public static int useForEachValue(ConcurrentHashMap<String, A> as, ConcurrentHashMap<String, B> bs) {
    as.forEachValue(1L, a -> a.run());
    bs.forEachValue(1L, b -> b.run());
    return 0;
  }
}
