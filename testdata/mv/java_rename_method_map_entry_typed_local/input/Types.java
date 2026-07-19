package demo;

import java.util.Map;
import java.util.NavigableMap;

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
  // Map.Entry typed param — direct getValue/setValue under foreign same-leaf.
  public static int useEntryParam(Map.Entry<String, A> ea, Map.Entry<String, B> eb) {
    return ea.getValue().run() + eb.getValue().run();
  }

  public static int useEntryParamSetValue(Map.Entry<String, A> ea, Map.Entry<String, B> eb) {
    return ea.setValue(new A()).run() + eb.setValue(new B()).run();
  }

  // Explicit Map.Entry local (non-var) — same declared-type path as params.
  public static int useEntryLocalDecl() {
    Map.Entry<String, A> ea = Map.entry("k", new A());
    Map.Entry<String, B> eb = Map.entry("k", new B());
    return ea.getValue().run() + eb.getValue().run();
  }

  // Enhanced-for with explicit Map.Entry type (not var).
  public static int useEntryFor(Map<String, A> am, Map<String, B> bm) {
    int n = 0;
    for (Map.Entry<String, A> ea : am.entrySet()) {
      n += ea.getValue().run();
    }
    for (Map.Entry<String, B> eb : bm.entrySet()) {
      n += eb.getValue().run();
    }
    return n;
  }

  // Assignment still works (regression).
  public static int useEntryAssign(Map.Entry<String, A> ea, Map.Entry<String, B> eb) {
    A xa = ea.getValue();
    B xb = eb.getValue();
    return xa.run() + xb.run();
  }

  // Fully-qualified java.util.Map.Entry — scoped head same as Map.Entry.
  public static int useQualifiedEntry(java.util.Map.Entry<String, A> ea, java.util.Map.Entry<String, B> eb) {
    return ea.getValue().run() + eb.getValue().run();
  }

  // Existing factory/var paths still work (regression).
  public static int useMapEntryVar() {
    var ea = Map.entry("k", new A());
    var eb = Map.entry("k", new B());
    return ea.getValue().run() + eb.getValue().run();
  }

  public static int useFirstEntry(NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    return am.firstEntry().getValue().run() + bm.firstEntry().getValue().run();
  }
}
