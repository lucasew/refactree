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
  public static int useGetChain(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.groupingBy(a -> "k"));
    var bm = bs.stream().collect(Collectors.groupingBy(b -> "k"));
    return am.get("k").get(0).run() + bm.get("k").get(0).run();
  }

  public static int useGetInline(List<A> as, List<B> bs) {
    return as.stream().collect(Collectors.groupingBy(a -> "k")).get("k").get(0).run()
        + bs.stream().collect(Collectors.groupingBy(b -> "k")).get("k").get(0).run();
  }

  public static int useGetOrDefault(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.groupingBy(a -> "k"));
    var bm = bs.stream().collect(Collectors.groupingBy(b -> "k"));
    return am.getOrDefault("k", List.of()).get(0).run()
        + bm.getOrDefault("k", List.of()).get(0).run();
  }

  public static int usePartitioningBy(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.partitioningBy(a -> true));
    var bm = bs.stream().collect(Collectors.partitioningBy(b -> true));
    return am.get(true).get(0).run() + bm.get(true).get(0).run();
  }

  public static int usePreservesB(List<B> bs) {
    var bm = bs.stream().collect(Collectors.groupingBy(b -> "k"));
    return bm.get("k").get(0).run();
  }
}
