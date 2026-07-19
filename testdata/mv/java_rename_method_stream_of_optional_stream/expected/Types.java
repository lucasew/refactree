package demo;

import java.util.List;
import java.util.Optional;
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
  public static int useStreamOfFlatMap(Optional<A> oa, Optional<B> ob) {
    return Stream.of(oa).flatMap(Optional::stream).mapToInt(x -> x.execute()).sum()
        + Stream.of(ob).flatMap(Optional::stream).mapToInt(y -> y.run()).sum();
  }

  public static int useStreamOfFlatMapForEach(Optional<A> oa, Optional<B> ob) {
    Stream.of(oa).flatMap(Optional::stream).forEach(x -> x.execute());
    Stream.of(ob).flatMap(Optional::stream).forEach(y -> y.run());
    return 0;
  }

  public static int useStreamOfFlatMapVar(Optional<A> oa, Optional<B> ob) {
    var sa = Stream.of(oa).flatMap(Optional::stream);
    var sb = Stream.of(ob).flatMap(Optional::stream);
    return sa.mapToInt(x -> x.execute()).sum() + sb.mapToInt(y -> y.run()).sum();
  }

  public static int useStreamOfFlatMapFindFirst(Optional<A> oa, Optional<B> ob, A da, B db) {
    var xa = Stream.of(oa).flatMap(Optional::stream).findFirst().orElse(da);
    var xb = Stream.of(ob).flatMap(Optional::stream).findFirst().orElse(db);
    return xa.execute() + xb.run();
  }

  public static int useOfCreationFlatMap() {
    return Stream.of(Optional.of(new A())).flatMap(Optional::stream).mapToInt(x -> x.execute()).sum()
        + Stream.of(Optional.of(new B())).flatMap(Optional::stream).mapToInt(y -> y.run()).sum();
  }

  public static int useLambdaStream(Optional<A> oa, Optional<B> ob) {
    return Stream.of(oa).flatMap(o -> o.stream()).mapToInt(x -> x.execute()).sum()
        + Stream.of(ob).flatMap(o -> o.stream()).mapToInt(y -> y.run()).sum();
  }

  public static int useListOptionalStream(List<Optional<A>> as, List<Optional<B>> bs) {
    return as.stream().flatMap(Optional::stream).mapToInt(x -> x.execute()).sum()
        + bs.stream().flatMap(Optional::stream).mapToInt(y -> y.run()).sum();
  }

  public static int useListOptionalLambda(List<Optional<A>> as, List<Optional<B>> bs) {
    return as.stream().flatMap(o -> o.stream()).mapToInt(x -> x.execute()).sum()
        + bs.stream().flatMap(o -> o.stream()).mapToInt(y -> y.run()).sum();
  }

  public static int usePreservesB(Optional<B> ob, List<Optional<B>> bs) {
    return Stream.of(ob).flatMap(Optional::stream).mapToInt(y -> y.run()).sum()
        + bs.stream().flatMap(Optional::stream).mapToInt(y -> y.run()).sum();
  }
}
