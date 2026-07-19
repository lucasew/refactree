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
  public static int useMapIfPresent(Optional<A> oa, Optional<B> ob) {
    oa.map(a -> a).ifPresent(x -> x.run());
    ob.map(b -> b).ifPresent(y -> y.run());
    return 0;
  }

  public static int useMapOrElse(Optional<A> oa, Optional<B> ob, A da, B db) {
    var xa = oa.map(a -> a).orElse(da);
    var xb = ob.map(b -> b).orElse(db);
    return xa.run() + xb.run();
  }

  public static int useMapLambdaParam(Optional<A> oa, Optional<B> ob) {
    oa.map(a -> { a.run(); return a; });
    ob.map(b -> { b.run(); return b; });
    return 0;
  }

  public static int useOfMap() {
    Optional.of(new A()).map(a -> a).ifPresent(x -> x.run());
    Optional.of(new B()).map(b -> b).ifPresent(y -> y.run());
    return 0;
  }

  public static int useMapNew() {
    Optional.of("x").map(s -> new A()).ifPresent(x -> x.run());
    Optional.of("y").map(s -> new B()).ifPresent(y -> y.run());
    return 0;
  }

  public static int useMapOrElseGet(Optional<A> oa, Optional<B> ob, A da, B db) {
    var xa = oa.map(a -> a).orElseGet(() -> da);
    var xb = ob.map(b -> b).orElseGet(() -> db);
    return xa.run() + xb.run();
  }
}
