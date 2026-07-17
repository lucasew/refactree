package demo;

import java.util.Optional;

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
  public static int useFlatMapIfPresent(Optional<A> oa, Optional<B> ob) {
    oa.flatMap(a -> Optional.of(a)).ifPresent(x -> x.run());
    ob.flatMap(b -> Optional.of(b)).ifPresent(y -> y.run());
    return 0;
  }

  public static int useFlatMapOrElse(Optional<A> oa, Optional<B> ob, A da, B db) {
    var xa = oa.flatMap(a -> Optional.of(a)).orElse(da);
    var xb = ob.flatMap(b -> Optional.of(b)).orElse(db);
    return xa.run() + xb.run();
  }

  public static int useFlatMapLambdaParam(Optional<A> oa, Optional<B> ob) {
    oa.flatMap(a -> { a.run(); return Optional.of(a); });
    ob.flatMap(b -> { b.run(); return Optional.of(b); });
    return 0;
  }

  public static int useOfFlatMap() {
    Optional.of(new A()).flatMap(a -> Optional.of(a)).ifPresent(x -> x.run());
    Optional.of(new B()).flatMap(b -> Optional.of(b)).ifPresent(y -> y.run());
    return 0;
  }

  public static int useFlatMapMethodRef(Optional<A> oa, Optional<B> ob) {
    oa.flatMap(Optional::of).ifPresent(x -> x.run());
    ob.flatMap(Optional::of).ifPresent(y -> y.run());
    return 0;
  }

  public static int useFlatMapOfNullable(Optional<A> oa, Optional<B> ob) {
    oa.flatMap(a -> Optional.ofNullable(a)).ifPresent(x -> x.run());
    ob.flatMap(b -> Optional.ofNullable(b)).ifPresent(y -> y.run());
    return 0;
  }
}
