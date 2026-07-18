package demo;

import java.util.HashMap;
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
  public static int useGroupingByToList(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.groupingBy(a -> "k", Collectors.toList())).get("k").get(0).execute();
    bs.stream().collect(Collectors.groupingBy(b -> "k", Collectors.toList())).get("k").get(0).run();
    return 0;
  }

  public static int useGroupingByToListVar(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.groupingBy(a -> "k", Collectors.toList()));
    var bm = bs.stream().collect(Collectors.groupingBy(b -> "k", Collectors.toList()));
    am.get("k").get(0).execute();
    bm.get("k").get(0).run();
    return 0;
  }

  public static int useGroupingByMapFactory(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.groupingBy(a -> "k", HashMap::new, Collectors.toList())).get("k").get(0).execute();
    bs.stream().collect(Collectors.groupingBy(b -> "k", HashMap::new, Collectors.toList())).get("k").get(0).run();
    return 0;
  }

  public static int usePartitioningByToList(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.partitioningBy(a -> true, Collectors.toList())).get(true).get(0).execute();
    bs.stream().collect(Collectors.partitioningBy(b -> true, Collectors.toList())).get(true).get(0).run();
    return 0;
  }

  public static int useCollectingAndThenGroupingBy(List<A> as, List<B> bs) {
    as.stream()
        .collect(Collectors.collectingAndThen(Collectors.groupingBy(a -> "k"), m -> m))
        .get("k")
        .get(0)
        .execute();
    bs.stream()
        .collect(Collectors.collectingAndThen(Collectors.groupingBy(b -> "k"), m -> m))
        .get("k")
        .get(0)
        .run();
    return 0;
  }

  public static int useCollectingAndThenVar(List<A> as, List<B> bs) {
    var am =
        as.stream()
            .collect(Collectors.collectingAndThen(Collectors.groupingBy(a -> "k"), m -> m));
    var bm =
        bs.stream()
            .collect(Collectors.collectingAndThen(Collectors.groupingBy(b -> "k"), m -> m));
    am.get("k").get(0).execute();
    bm.get("k").get(0).run();
    return 0;
  }

  public static int useValuesForEach(List<A> as, List<B> bs) {
    as.stream()
        .collect(Collectors.groupingBy(a -> "k", Collectors.toList()))
        .values()
        .forEach(g -> g.forEach(a -> a.execute()));
    bs.stream()
        .collect(Collectors.groupingBy(b -> "k", Collectors.toList()))
        .values()
        .forEach(g -> g.forEach(b -> b.run()));
    return 0;
  }


  public static int useGroupingByToSet(List<A> as, List<B> bs) {
    as.stream()
        .collect(Collectors.groupingBy(a -> "k", Collectors.toSet()))
        .get("k")
        .forEach(a -> a.execute());
    bs.stream()
        .collect(Collectors.groupingBy(b -> "k", Collectors.toSet()))
        .get("k")
        .forEach(b -> b.run());
    return 0;
  }

  public static int useGroupingByMappingIdentity(List<A> as, List<B> bs) {
    as.stream()
        .collect(Collectors.groupingBy(a -> "k", Collectors.mapping(x -> x, Collectors.toList())))
        .get("k")
        .get(0)
        .execute();
    bs.stream()
        .collect(Collectors.groupingBy(b -> "k", Collectors.mapping(x -> x, Collectors.toList())))
        .get("k")
        .get(0)
        .run();
    return 0;
  }

  public static int usePreservesB(List<B> bs) {
    return bs.stream()
        .collect(Collectors.groupingBy(b -> "k", Collectors.toList()))
        .get("k")
        .get(0)
        .run();
  }
}
