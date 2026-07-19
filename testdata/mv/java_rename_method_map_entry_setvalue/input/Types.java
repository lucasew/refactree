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
  public static int useEntrySetValue(Map.Entry<String, A> ea, Map.Entry<String, B> eb) {
    var va = ea.setValue(new A());
    var vb = eb.setValue(new B());
    return va.run() + vb.run();
  }

  public static int useEntrySetValueInline(Map.Entry<String, A> ea, Map.Entry<String, B> eb) {
    return ea.setValue(new A()).run() + eb.setValue(new B()).run();
  }

  public static int useFirstEntrySetValue(NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    var va = am.firstEntry().setValue(new A());
    var vb = bm.firstEntry().setValue(new B());
    return va.run() + vb.run();
  }

  public static int useFirstEntryVarSetValue(NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    var ea = am.firstEntry();
    var eb = bm.firstEntry();
    var va = ea.setValue(new A());
    var vb = eb.setValue(new B());
    return va.run() + vb.run();
  }

  public static int useLastEntrySetValue(NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    var ea = am.lastEntry();
    var eb = bm.lastEntry();
    return ea.setValue(new A()).run() + eb.setValue(new B()).run();
  }

  public static int useEntrySetForVar(Map<String, A> am, Map<String, B> bm) {
    int n = 0;
    for (var e : am.entrySet()) {
      var va = e.setValue(new A());
      n += va.run();
    }
    for (var e : bm.entrySet()) {
      var vb = e.setValue(new B());
      n += vb.run();
    }
    return n;
  }
}
