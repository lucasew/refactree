package demo;

import java.util.Collection;
import java.util.List;

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
  public static int useFlatMapColl(List<List<A>> nestedA, List<List<B>> nestedB) {
    return nestedA.stream().flatMap(Collection::stream).mapToInt(x -> x.run()).sum()
        + nestedB.stream().flatMap(Collection::stream).mapToInt(y -> y.run()).sum();
  }

  public static int useFlatMapList(List<List<A>> nestedA, List<List<B>> nestedB) {
    return nestedA.stream().flatMap(List::stream).mapToInt(x -> x.run()).sum()
        + nestedB.stream().flatMap(List::stream).mapToInt(y -> y.run()).sum();
  }

  public static int useFlatMapLambda(List<List<A>> nestedA, List<List<B>> nestedB) {
    return nestedA.stream().flatMap(xs -> xs.stream()).mapToInt(x -> x.run()).sum()
        + nestedB.stream().flatMap(ys -> ys.stream()).mapToInt(y -> y.run()).sum();
  }

  public static int useFlatMapForEach(List<List<A>> nestedA, List<List<B>> nestedB) {
    nestedA.stream().flatMap(Collection::stream).forEach(x -> x.run());
    nestedB.stream().flatMap(Collection::stream).forEach(y -> y.run());
    return 0;
  }

  public static int useFlatMapVar(List<List<A>> nestedA, List<List<B>> nestedB) {
    var sa = nestedA.stream().flatMap(Collection::stream);
    var sb = nestedB.stream().flatMap(Collection::stream);
    return sa.mapToInt(x -> x.run()).sum() + sb.mapToInt(y -> y.run()).sum();
  }

  public static int usePreservesB(List<List<B>> nestedB) {
    return nestedB.stream().flatMap(Collection::stream).mapToInt(y -> y.run()).sum();
  }
}
