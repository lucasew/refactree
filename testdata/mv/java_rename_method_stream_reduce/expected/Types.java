package demo;

import java.util.List;

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
  public static int useReduceIdentity(List<A> as, List<B> bs, A da, B db) {
    var xa = as.stream().reduce(da, (x, y) -> x);
    var xb = bs.stream().reduce(db, (x, y) -> x);
    return xa.execute() + xb.run();
  }

  public static int useReduceOptional(List<A> as, List<B> bs, A da, B db) {
    var xa = as.stream().reduce((x, y) -> x).orElse(da);
    var xb = bs.stream().reduce((x, y) -> x).orElse(db);
    return xa.execute() + xb.run();
  }

  public static int useReduceIfPresent(List<A> as, List<B> bs) {
    as.stream().reduce((x, y) -> x).ifPresent(a -> a.execute());
    bs.stream().reduce((x, y) -> x).ifPresent(b -> b.run());
    return 0;
  }

  public static int useReduceCombiner(List<A> as, List<B> bs, A da, B db) {
    var xa = as.stream().reduce(da, (x, y) -> x, (x, y) -> x);
    var xb = bs.stream().reduce(db, (x, y) -> x, (x, y) -> x);
    return xa.execute() + xb.run();
  }
}
