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
  public static int useConcurrentValues(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.toConcurrentMap(a -> "k", a -> a)).values().forEach(a -> a.execute());
    bs.stream().collect(Collectors.toConcurrentMap(b -> "k", b -> b)).values().forEach(b -> b.run());
    return 0;
  }

  public static int useConcurrentVar(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.toConcurrentMap(a -> "k", a -> a));
    var bm = bs.stream().collect(Collectors.toConcurrentMap(b -> "k", b -> b));
    am.values().forEach(a -> a.execute());
    bm.values().forEach(b -> b.run());
    am.forEach((k, a) -> a.execute());
    bm.forEach((k, b) -> b.run());
    var xa = am.get("k");
    var xb = bm.get("k");
    xa.execute();
    xb.run();
    return 0;
  }

  public static int useConcurrentMerge(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.toConcurrentMap(a -> "k", a -> a, (x, y) -> x)).values().forEach(a -> a.execute());
    bs.stream().collect(Collectors.toConcurrentMap(b -> "k", b -> b, (x, y) -> x)).values().forEach(b -> b.run());
    return 0;
  }

  public static int useUnmodifiableValues(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.toUnmodifiableMap(a -> "k", a -> a)).values().forEach(a -> a.execute());
    bs.stream().collect(Collectors.toUnmodifiableMap(b -> "k", b -> b)).values().forEach(b -> b.run());
    return 0;
  }

  public static int useUnmodifiableVar(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.toUnmodifiableMap(a -> "k", a -> a));
    var bm = bs.stream().collect(Collectors.toUnmodifiableMap(b -> "k", b -> b));
    am.values().forEach(a -> a.execute());
    bm.values().forEach(b -> b.run());
    am.forEach((k, a) -> a.execute());
    bm.forEach((k, b) -> b.run());
    var xa = am.get("k");
    var xb = bm.get("k");
    xa.execute();
    xb.run();
    return 0;
  }

  public static int useUnmodifiableMerge(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.toUnmodifiableMap(a -> "k", a -> a, (x, y) -> x)).values().forEach(a -> a.execute());
    bs.stream().collect(Collectors.toUnmodifiableMap(b -> "k", b -> b, (x, y) -> x)).values().forEach(b -> b.run());
    return 0;
  }
}
