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
  public static int useMapEntryInline() {
    return Map.entry("k", new A()).getValue().run()
        + Map.entry("k", new B()).getValue().run();
  }

  public static int useMapEntryVar() {
    var ea = Map.entry("k", new A());
    var eb = Map.entry("k", new B());
    return ea.getValue().run() + eb.getValue().run();
  }

  public static int useMapEntryGetValueVar() {
    var va = Map.entry("k", new A()).getValue();
    var vb = Map.entry("k", new B()).getValue();
    return va.run() + vb.run();
  }

  public static int useFirstEntry(NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    return am.firstEntry().getValue().run() + bm.firstEntry().getValue().run();
  }

  public static int useFirstEntryVar(NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    var ea = am.firstEntry();
    var eb = bm.firstEntry();
    return ea.getValue().run() + eb.getValue().run();
  }

  public static int useFirstEntryGetValueVar(NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    var va = am.firstEntry().getValue();
    var vb = bm.firstEntry().getValue();
    return va.run() + vb.run();
  }

  public static int useLastEntry(NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    var ea = am.lastEntry();
    var eb = bm.lastEntry();
    return ea.getValue().run() + eb.getValue().run();
  }
}
