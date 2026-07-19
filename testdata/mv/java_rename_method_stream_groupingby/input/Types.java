package demo;

import java.util.List;
import java.util.stream.Collectors;

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
  public static int useValuesForEach(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.groupingBy(a -> "k")).values().forEach(g -> g.forEach(a -> a.run()));
    bs.stream().collect(Collectors.groupingBy(b -> "k")).values().forEach(g -> g.forEach(b -> b.run()));
    return 0;
  }

  public static int useMapForEach(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.groupingBy(a -> "k")).forEach((k, g) -> g.forEach(a -> a.run()));
    bs.stream().collect(Collectors.groupingBy(b -> "k")).forEach((k, g) -> g.forEach(b -> b.run()));
    return 0;
  }

  public static int useVarValues(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.groupingBy(a -> "k"));
    var bm = bs.stream().collect(Collectors.groupingBy(b -> "k"));
    am.values().forEach(g -> g.forEach(a -> a.run()));
    bm.values().forEach(g -> g.forEach(b -> b.run()));
    int n = 0;
    for (var g : am.values()) {
      for (var a : g) {
        n += a.run();
      }
    }
    for (var g : bm.values()) {
      for (var b : g) {
        n += b.run();
      }
    }
    return n;
  }

  public static int useVarGet(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.groupingBy(a -> "k"));
    var bm = bs.stream().collect(Collectors.groupingBy(b -> "k"));
    var ga = am.get("k");
    var gb = bm.get("k");
    ga.forEach(a -> a.run());
    gb.forEach(b -> b.run());
    return 0;
  }

  public static int useVarMapForEach(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.groupingBy(a -> "k"));
    var bm = bs.stream().collect(Collectors.groupingBy(b -> "k"));
    am.forEach((k, g) -> g.forEach(a -> a.run()));
    bm.forEach((k, g) -> g.forEach(b -> b.run()));
    return 0;
  }
}
