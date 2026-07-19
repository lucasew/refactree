package demo;

import java.util.LinkedHashMap;
import java.util.SequencedMap;
import java.util.TreeMap;

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
  public static int useReversedGet(LinkedHashMap<String, A> as, LinkedHashMap<String, B> bs) {
    var xa = as.reversed().get("k");
    var xb = bs.reversed().get("k");
    return xa.execute() + xb.run();
  }

  public static int useReversedValues(LinkedHashMap<String, A> as, LinkedHashMap<String, B> bs) {
    as.reversed().values().forEach(a -> a.execute());
    bs.reversed().values().forEach(b -> b.run());
    return 0;
  }

  public static int useReversedForEach(LinkedHashMap<String, A> as, LinkedHashMap<String, B> bs) {
    as.reversed().forEach((k, a) -> a.execute());
    bs.reversed().forEach((k, b) -> b.run());
    return 0;
  }

  public static int useReversedEntrySet(LinkedHashMap<String, A> as, LinkedHashMap<String, B> bs) {
    as.reversed().entrySet().forEach(ea -> ea.getValue().execute());
    bs.reversed().entrySet().forEach(eb -> eb.getValue().run());
    return 0;
  }

  public static int useVarReversed(LinkedHashMap<String, A> as, LinkedHashMap<String, B> bs) {
    var am = as.reversed();
    var bm = bs.reversed();
    var xa = am.get("k");
    var xb = bm.get("k");
    return xa.execute() + xb.run();
  }

  public static int useSequencedMapParam(SequencedMap<String, A> as, SequencedMap<String, B> bs) {
    var xa = as.reversed().get("k");
    var xb = bs.reversed().get("k");
    return xa.execute() + xb.run();
  }

  public static int useTreeMapReversed(TreeMap<String, A> as, TreeMap<String, B> bs) {
    as.reversed().values().forEach(a -> a.execute());
    bs.reversed().values().forEach(b -> b.run());
    return 0;
  }
}
