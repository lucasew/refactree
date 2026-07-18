package demo;

import java.util.NavigableMap;
import java.util.SortedMap;
import java.util.TreeMap;

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
  // Chain: descendingMap is value-type-preserving like SequencedMap.reversed.
  public static int useDescendingMapGetChain(
      NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    return am.descendingMap().get("k").run() + bm.descendingMap().get("k").run();
  }

  public static int useHeadMapGetChain(
      NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    return am.headMap("z").get("k").run() + bm.headMap("z").get("k").run();
  }

  public static int useTailMapGetChain(
      NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    return am.tailMap("a").get("k").run() + bm.tailMap("a").get("k").run();
  }

  public static int useSubMapGetChain(
      NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    return am.subMap("a", "z").get("k").run() + bm.subMap("a", "z").get("k").run();
  }

  // NavigableMap overloads with inclusivity flags (bounds only; same V).
  public static int useHeadMapInclusiveChain(
      NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    return am.headMap("z", true).get("k").run() + bm.headMap("z", true).get("k").run();
  }

  public static int useTailMapInclusiveChain(
      NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    return am.tailMap("a", false).get("k").run() + bm.tailMap("a", false).get("k").run();
  }

  public static int useSubMapInclusiveChain(
      NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    return am.subMap("a", true, "z", false).get("k").run()
        + bm.subMap("a", true, "z", false).get("k").run();
  }

  // var from view then get — value leaf through valOf.
  public static int useVarDescendingMap(
      NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    var dm = am.descendingMap();
    var em = bm.descendingMap();
    var xa = dm.get("k");
    var xb = em.get("k");
    return xa.run() + xb.run();
  }

  public static int useVarHeadTailSub(
      NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    var hm = am.headMap("z");
    var tm = bm.tailMap("a");
    var sm = am.subMap("a", "z");
    return hm.get("k").run() + tm.get("k").run() + sm.get("k").run();
  }

  // SortedMap views (same V leaf).
  public static int useSortedMapViews(
      SortedMap<String, A> am, SortedMap<String, B> bm) {
    return am.headMap("z").get("k").run()
        + bm.tailMap("a").get("k").run()
        + am.subMap("a", "z").get("k").run();
  }

  // values / forEach / entrySet through view.
  public static int useDescendingMapValues(TreeMap<String, A> am, TreeMap<String, B> bm) {
    am.descendingMap().values().forEach(a -> a.run());
    bm.descendingMap().values().forEach(b -> b.run());
    return 0;
  }

  public static int useHeadMapForEach(
      NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    am.headMap("z").forEach((k, a) -> a.run());
    bm.headMap("z").forEach((k, b) -> b.run());
    return 0;
  }

  public static int useSubMapEntrySet(
      NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    am.subMap("a", "z").entrySet().forEach(ea -> ea.getValue().run());
    bm.subMap("a", "z").entrySet().forEach(eb -> eb.getValue().run());
    return 0;
  }

  // Regression: plain get / reversed path neighbors.
  public static int usePlainGet(NavigableMap<String, A> am, NavigableMap<String, B> bm) {
    var xa = am.get("k");
    var xb = bm.get("k");
    return xa.run() + xb.run();
  }
}
