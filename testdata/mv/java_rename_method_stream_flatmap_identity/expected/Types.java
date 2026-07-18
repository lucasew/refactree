package demo;

import java.util.List;
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
  public static int useFlatMapOf(List<A> as, List<B> bs) {
    return as.stream().flatMap(a -> Stream.of(a)).mapToInt(x -> x.execute()).sum()
        + bs.stream().flatMap(b -> Stream.of(b)).mapToInt(y -> y.run()).sum();
  }

  public static int useFlatMapForEach(List<A> as, List<B> bs) {
    as.stream().flatMap(a -> Stream.of(a)).forEach(x -> x.execute());
    bs.stream().flatMap(b -> Stream.of(b)).forEach(y -> y.run());
    return 0;
  }

  public static int useFlatMapVar(List<A> as, List<B> bs) {
    var sa = as.stream().flatMap(a -> Stream.of(a));
    var sb = bs.stream().flatMap(b -> Stream.of(b));
    return sa.mapToInt(x -> x.execute()).sum() + sb.mapToInt(y -> y.run()).sum();
  }

  public static int useFlatMapFindFirst(List<A> as, List<B> bs, A da, B db) {
    var xa = as.stream().flatMap(a -> Stream.of(a)).findFirst().orElse(da);
    var xb = bs.stream().flatMap(b -> Stream.of(b)).findFirst().orElse(db);
    return xa.execute() + xb.run();
  }

  public static int useOfFlatMap() {
    return Stream.of(new A()).flatMap(a -> Stream.of(a)).mapToInt(x -> x.execute()).sum()
        + Stream.of(new B()).flatMap(b -> Stream.of(b)).mapToInt(y -> y.run()).sum();
  }

  public static int useFlatMapMethodRef(List<A> as, List<B> bs) {
    return as.stream().flatMap(Stream::of).mapToInt(x -> x.execute()).sum()
        + bs.stream().flatMap(Stream::of).mapToInt(y -> y.run()).sum();
  }

  public static int usePreservesB(List<B> bs) {
    return bs.stream().flatMap(b -> Stream.of(b)).mapToInt(y -> y.run()).sum();
  }
}
