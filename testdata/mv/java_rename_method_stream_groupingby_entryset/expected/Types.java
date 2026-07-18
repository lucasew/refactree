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
  public static int useEntrySetForEach(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.groupingBy(a -> "k"));
    var bm = bs.stream().collect(Collectors.groupingBy(b -> "k"));
    am.entrySet().forEach(ea -> ea.getValue().forEach(a -> a.execute()));
    bm.entrySet().forEach(eb -> eb.getValue().forEach(b -> b.run()));
    return 0;
  }

  public static int useEntrySetForVar(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.groupingBy(a -> "k"));
    var bm = bs.stream().collect(Collectors.groupingBy(b -> "k"));
    int n = 0;
    for (var ea : am.entrySet()) {
      for (var a : ea.getValue()) {
        n += a.execute();
      }
    }
    for (var eb : bm.entrySet()) {
      for (var b : eb.getValue()) {
        n += b.run();
      }
    }
    return n;
  }

  public static int useEntrySetGetChain(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.groupingBy(a -> "k"));
    var bm = bs.stream().collect(Collectors.groupingBy(b -> "k"));
    return am.entrySet().stream().findFirst().get().getValue().get(0).execute()
        + bm.entrySet().stream().findFirst().get().getValue().get(0).run();
  }

  public static int useEntrySetGetValueVar(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.groupingBy(a -> "k"));
    var bm = bs.stream().collect(Collectors.groupingBy(b -> "k"));
    int n = 0;
    for (var ea : am.entrySet()) {
      var ga = ea.getValue();
      n += ga.get(0).execute();
      ga.forEach(a -> a.execute());
    }
    for (var eb : bm.entrySet()) {
      var gb = eb.getValue();
      n += gb.get(0).run();
      gb.forEach(b -> b.run());
    }
    return n;
  }

  public static int useEntrySetInline(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.groupingBy(a -> "k")).entrySet()
        .forEach(ea -> ea.getValue().get(0).execute());
    bs.stream().collect(Collectors.groupingBy(b -> "k")).entrySet()
        .forEach(eb -> eb.getValue().get(0).run());
    return 0;
  }

  public static int usePartitioningByEntrySet(List<A> as, List<B> bs) {
    var am = as.stream().collect(Collectors.partitioningBy(a -> true));
    var bm = bs.stream().collect(Collectors.partitioningBy(b -> true));
    am.entrySet().forEach(ea -> ea.getValue().forEach(a -> a.execute()));
    bm.entrySet().forEach(eb -> eb.getValue().forEach(b -> b.run()));
    return 0;
  }

  public static int usePreservesB(List<B> bs) {
    var bm = bs.stream().collect(Collectors.groupingBy(b -> "k"));
    bm.entrySet().forEach(eb -> eb.getValue().forEach(b -> b.run()));
    for (var eb : bm.entrySet()) {
      for (var b : eb.getValue()) {
        b.run();
      }
    }
    return bm.entrySet().stream().findFirst().get().getValue().get(0).run();
  }
}
