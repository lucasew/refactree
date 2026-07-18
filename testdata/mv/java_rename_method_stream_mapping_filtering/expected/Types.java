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
  public static int useFilteringForEach(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.filtering(a -> true, Collectors.toList())).forEach(a -> a.execute());
    bs.stream().collect(Collectors.filtering(b -> true, Collectors.toList())).forEach(b -> b.run());
    return 0;
  }

  public static int useMappingForEach(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.mapping(a -> a, Collectors.toList())).forEach(a -> a.execute());
    bs.stream().collect(Collectors.mapping(b -> b, Collectors.toList())).forEach(b -> b.run());
    return 0;
  }

  public static int useFilteringToSet(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.filtering(a -> true, Collectors.toSet())).forEach(a -> a.execute());
    bs.stream().collect(Collectors.filtering(b -> true, Collectors.toSet())).forEach(b -> b.run());
    return 0;
  }

  public static int useMappingToSet(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.mapping(a -> a, Collectors.toSet())).forEach(a -> a.execute());
    bs.stream().collect(Collectors.mapping(b -> b, Collectors.toSet())).forEach(b -> b.run());
    return 0;
  }

  public static int useVarFiltering(List<A> as, List<B> bs) {
    var al = as.stream().collect(Collectors.filtering(a -> true, Collectors.toList()));
    var bl = bs.stream().collect(Collectors.filtering(b -> true, Collectors.toList()));
    al.forEach(a -> a.execute());
    bl.forEach(b -> b.run());
    int n = 0;
    for (var a : al) {
      n += a.execute();
    }
    for (var b : bl) {
      n += b.run();
    }
    return n;
  }

  public static int useVarMapping(List<A> as, List<B> bs) {
    var al = as.stream().collect(Collectors.mapping(a -> a, Collectors.toList()));
    var bl = bs.stream().collect(Collectors.mapping(b -> b, Collectors.toList()));
    al.forEach(a -> a.execute());
    bl.forEach(b -> b.run());
    int n = 0;
    for (var a : al) {
      n += a.execute();
    }
    for (var b : bl) {
      n += b.run();
    }
    return n;
  }
}
