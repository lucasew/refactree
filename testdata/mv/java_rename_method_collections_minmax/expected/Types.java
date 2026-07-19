package demo;

import java.util.Collections;
import java.util.Comparator;
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
  public static int useCollectionsMin(List<A> as, List<B> bs) {
    var xa = Collections.min(as, Comparator.comparingInt(x -> 0));
    var xb = Collections.min(bs, Comparator.comparingInt(x -> 0));
    return xa.execute() + xb.run();
  }

  public static int useCollectionsMax(List<A> as, List<B> bs) {
    var xa = Collections.max(as, Comparator.comparingInt(x -> 0));
    var xb = Collections.max(bs, Comparator.comparingInt(x -> 0));
    return xa.execute() + xb.run();
  }

  public static int useStreamMinOrElse(List<A> as, List<B> bs, A da, B db) {
    var xa = as.stream().min(Comparator.comparingInt(x -> 0)).orElse(da);
    var xb = bs.stream().min(Comparator.comparingInt(x -> 0)).orElse(db);
    return xa.execute() + xb.run();
  }

  public static int useStreamMaxOrElse(List<A> as, List<B> bs, A da, B db) {
    var xa = as.stream().max(Comparator.comparingInt(x -> 0)).orElse(da);
    var xb = bs.stream().max(Comparator.comparingInt(x -> 0)).orElse(db);
    return xa.execute() + xb.run();
  }

  public static int useStreamMinIfPresent(List<A> as, List<B> bs) {
    as.stream().min(Comparator.comparingInt(x -> 0)).ifPresent(a -> a.execute());
    bs.stream().max(Comparator.comparingInt(x -> 0)).ifPresent(b -> b.run());
    return 0;
  }
}
