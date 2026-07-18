package demo;

import java.util.Collections;
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
    as.stream().collect(Collectors.collectingAndThen(Collectors.toMap(a -> "k", a -> a), m -> m)).values().forEach(a -> a.execute());
    bs.stream().collect(Collectors.collectingAndThen(Collectors.toMap(b -> "k", b -> b), m -> m)).values().forEach(b -> b.run());
    return 0;
  }

  public static int useMapForEach(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.collectingAndThen(Collectors.toMap(a -> "k", a -> a), m -> m)).forEach((k, a) -> a.execute());
    bs.stream().collect(Collectors.collectingAndThen(Collectors.toMap(b -> "k", b -> b), m -> m)).forEach((k, b) -> b.run());
    return 0;
  }

  public static int useUnmodifiable(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.collectingAndThen(Collectors.toMap(a -> "k", a -> a), Collections::unmodifiableMap)).values().forEach(a -> a.execute());
    bs.stream().collect(Collectors.collectingAndThen(Collectors.toMap(b -> "k", b -> b), Collections::unmodifiableMap)).values().forEach(b -> b.run());
    return 0;
  }

  public static int useVarValues(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.collectingAndThen(Collectors.toMap(a -> "k", a -> a), m -> m));
    var bm = bs.stream().collect(Collectors.collectingAndThen(Collectors.toMap(b -> "k", b -> b), m -> m));
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
    var am = as.stream().collect(Collectors.collectingAndThen(Collectors.toMap(a -> "k", a -> a), m -> m));
    var bm = bs.stream().collect(Collectors.collectingAndThen(Collectors.toMap(b -> "k", b -> b), m -> m));
    var xa = am.get("k");
    var xb = bm.get("k");
    xa.execute();
    xb.run();
    return 0;
  }

  public static int useVarMapForEach(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.collectingAndThen(Collectors.toMap(a -> "k", a -> a), m -> m));
    var bm = bs.stream().collect(Collectors.collectingAndThen(Collectors.toMap(b -> "k", b -> b), m -> m));
    am.forEach((k, a) -> a.execute());
    bm.forEach((k, b) -> b.run());
    return 0;
  }

  public static int useConcurrent(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.collectingAndThen(Collectors.toConcurrentMap(a -> "k", a -> a), m -> m)).values().forEach(a -> a.execute());
    bs.stream().collect(Collectors.collectingAndThen(Collectors.toConcurrentMap(b -> "k", b -> b), m -> m)).values().forEach(b -> b.run());
    return 0;
  }

  public static int useMerge(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.collectingAndThen(Collectors.toMap(a -> "k", a -> a, (x, y) -> x), m -> m)).values().forEach(a -> a.execute());
    bs.stream().collect(Collectors.collectingAndThen(Collectors.toMap(b -> "k", b -> b, (x, y) -> x), m -> m)).values().forEach(b -> b.run());
    return 0;
  }
}
