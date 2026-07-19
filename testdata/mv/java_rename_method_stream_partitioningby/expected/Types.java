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
    as.stream().collect(Collectors.partitioningBy(a -> true)).values().forEach(g -> g.forEach(a -> a.execute()));
    bs.stream().collect(Collectors.partitioningBy(b -> true)).values().forEach(g -> g.forEach(b -> b.run()));
    return 0;
  }

  public static int useMapForEach(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.partitioningBy(a -> true)).forEach((k, g) -> g.forEach(a -> a.execute()));
    bs.stream().collect(Collectors.partitioningBy(b -> true)).forEach((k, g) -> g.forEach(b -> b.run()));
    return 0;
  }

  public static int useVarValues(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.partitioningBy(a -> true));
    var bm = bs.stream().collect(Collectors.partitioningBy(b -> true));
    am.values().forEach(g -> g.forEach(a -> a.execute()));
    bm.values().forEach(g -> g.forEach(b -> b.run()));
    int n = 0;
    for (var g : am.values()) {
      for (var a : g) {
        n += a.execute();
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
    var am = as.stream().collect(Collectors.partitioningBy(a -> true));
    var bm = bs.stream().collect(Collectors.partitioningBy(b -> true));
    var ga = am.get(true);
    var gb = bm.get(true);
    ga.forEach(a -> a.execute());
    gb.forEach(b -> b.run());
    return 0;
  }

  public static int useVarMapForEach(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.partitioningBy(a -> true));
    var bm = bs.stream().collect(Collectors.partitioningBy(b -> true));
    am.forEach((k, g) -> g.forEach(a -> a.execute()));
    bm.forEach((k, g) -> g.forEach(b -> b.run()));
    return 0;
  }
}
