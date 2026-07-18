package demo;

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
  public static int usePollFirstEntry(NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    return am.pollFirstEntry().getValue().run() + bm.pollFirstEntry().getValue().run();
  }

  public static int usePollLastEntry(NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    var ea = am.pollLastEntry();
    var eb = bm.pollLastEntry();
    return ea.getValue().run() + eb.getValue().run();
  }

  public static int useCeilingEntry(NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    var va = am.ceilingEntry("k").getValue();
    var vb = bm.ceilingEntry("k").getValue();
    return va.run() + vb.run();
  }

  public static int useFloorHigherLower(
      NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    var fa = am.floorEntry("k");
    var hb = bm.higherEntry("k");
    var la = am.lowerEntry("k");
    return fa.getValue().run() + hb.getValue().run() + la.getValue().run();
  }
}
