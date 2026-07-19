package demo;

import java.util.List;
import java.util.stream.Collectors;

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
  public static int useValuesForEach(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.toMap(a -> "k", a -> a)).values().forEach(a -> a.execute());
    bs.stream().collect(Collectors.toMap(b -> "k", b -> b)).values().forEach(b -> b.run());
    return 0;
  }

  public static int useMapForEach(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.toMap(a -> "k", a -> a)).forEach((k, a) -> a.execute());
    bs.stream().collect(Collectors.toMap(b -> "k", b -> b)).forEach((k, b) -> b.run());
    return 0;
  }

  public static int useVarValues(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.toMap(a -> "k", a -> a));
    var bm = bs.stream().collect(Collectors.toMap(b -> "k", b -> b));
    am.values().forEach(a -> a.execute());
    bm.values().forEach(b -> b.run());
    int n = 0;
    for (var a : am.values()) {
      n += a.execute();
    }
    for (var b : bm.values()) {
      n += b.run();
    }
    return n;
  }

  public static int useVarGet(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.toMap(a -> "k", a -> a));
    var bm = bs.stream().collect(Collectors.toMap(b -> "k", b -> b));
    var xa = am.get("k");
    var xb = bm.get("k");
    xa.execute();
    xb.run();
    return 0;
  }

  public static int useVarMapForEach(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.toMap(a -> "k", a -> a));
    var bm = bs.stream().collect(Collectors.toMap(b -> "k", b -> b));
    am.forEach((k, a) -> a.execute());
    bm.forEach((k, b) -> b.run());
    return 0;
  }

  public static int useMerge(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.toMap(a -> "k", a -> a, (x, y) -> x)).values().forEach(a -> a.execute());
    bs.stream().collect(Collectors.toMap(b -> "k", b -> b, (x, y) -> x)).values().forEach(b -> b.run());
    return 0;
  }
}
