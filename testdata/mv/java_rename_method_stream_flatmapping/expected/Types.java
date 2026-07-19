package demo;

import java.util.List;
import java.util.stream.Collectors;
import java.util.stream.Stream;

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
  public static int useFlatMappingForEach(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.flatMapping(a -> Stream.of(a), Collectors.toList())).forEach(a -> a.execute());
    bs.stream().collect(Collectors.flatMapping(b -> Stream.of(b), Collectors.toList())).forEach(b -> b.run());
    return 0;
  }

  public static int useFlatMappingMethodRef(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.flatMapping(Stream::of, Collectors.toList())).forEach(a -> a.execute());
    bs.stream().collect(Collectors.flatMapping(Stream::of, Collectors.toList())).forEach(b -> b.run());
    return 0;
  }

  public static int useFlatMappingOfNullable(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.flatMapping(a -> Stream.ofNullable(a), Collectors.toList())).forEach(a -> a.execute());
    bs.stream().collect(Collectors.flatMapping(b -> Stream.ofNullable(b), Collectors.toList())).forEach(b -> b.run());
    return 0;
  }

  public static int useFlatMappingToSet(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.flatMapping(a -> Stream.of(a), Collectors.toSet())).forEach(a -> a.execute());
    bs.stream().collect(Collectors.flatMapping(b -> Stream.of(b), Collectors.toSet())).forEach(b -> b.run());
    return 0;
  }

  public static int useVarFlatMapping(List<A> as, List<B> bs) {
    var al = as.stream().collect(Collectors.flatMapping(a -> Stream.of(a), Collectors.toList()));
    var bl = bs.stream().collect(Collectors.flatMapping(b -> Stream.of(b), Collectors.toList()));
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
