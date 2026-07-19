package demo;

import java.util.Comparator;
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
  public static int useReducingIdentity(List<A> as, List<B> bs, A da, B db) {
    var xa = as.stream().collect(Collectors.reducing(da, (x, y) -> x));
    var xb = bs.stream().collect(Collectors.reducing(db, (x, y) -> x));
    return xa.execute() + xb.run();
  }

  public static int useReducingOptional(List<A> as, List<B> bs, A da, B db) {
    var xa = as.stream().collect(Collectors.reducing((x, y) -> x)).orElse(da);
    var xb = bs.stream().collect(Collectors.reducing((x, y) -> x)).orElse(db);
    return xa.execute() + xb.run();
  }

  public static int useReducingIfPresent(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.reducing((x, y) -> x)).ifPresent(a -> a.execute());
    bs.stream().collect(Collectors.reducing((x, y) -> x)).ifPresent(b -> b.run());
    return 0;
  }

  public static int useMaxByIfPresent(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.maxBy(Comparator.comparingInt(x -> 0))).ifPresent(a -> a.execute());
    bs.stream().collect(Collectors.minBy(Comparator.comparingInt(x -> 0))).ifPresent(b -> b.run());
    return 0;
  }

  public static int useMaxByOrElse(List<A> as, List<B> bs, A da, B db) {
    var xa = as.stream().collect(Collectors.maxBy(Comparator.comparingInt(x -> 0))).orElse(da);
    var xb = bs.stream().collect(Collectors.minBy(Comparator.comparingInt(x -> 0))).orElse(db);
    return xa.execute() + xb.run();
  }

  public static int useReducingBody(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.reducing((a1, a2) -> { a1.execute(); return a1; }));
    bs.stream().collect(Collectors.reducing((b1, b2) -> { b1.run(); return b1; }));
    return 0;
  }
}
