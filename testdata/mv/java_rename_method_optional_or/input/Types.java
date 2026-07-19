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
  // Optional.or always preserves T — under foreign same-leaf.
  // Isolated: unique lambda/var names so file-scoped typedLocals cannot mask.
  public static int useOrIfPresent(Optional<A> oa, Optional<B> ob) {
    oa.or(() -> Optional.empty()).ifPresent(x1 -> x1.run());
    ob.or(() -> Optional.empty()).ifPresent(y1 -> y1.run());
    return 0;
  }

  public static int useOrOrElse(Optional<A> oa, Optional<B> ob, A da, B db) {
    var xa1 = oa.or(() -> Optional.of(da)).orElse(da);
    var xb1 = ob.or(() -> Optional.of(db)).orElse(db);
    return xa1.run() + xb1.run();
  }

  public static int useOrOrElseGet(Optional<A> oa, Optional<B> ob) {
    var xa2 = oa.or(() -> Optional.empty()).orElseGet(() -> new A());
    var xb2 = ob.or(() -> Optional.empty()).orElseGet(() -> new B());
    return xa2.run() + xb2.run();
  }

  public static int useOrOrElseThrow(Optional<A> oa, Optional<B> ob) {
    var xa3 = oa.or(() -> Optional.empty()).orElseThrow();
    var xb3 = ob.or(() -> Optional.empty()).orElseThrow();
    return xa3.run() + xb3.run();
  }

  // or bound to var, then ifPresent / orElse.
  public static int useOrVarIfPresent(Optional<A> oa, Optional<B> ob) {
    var oa2 = oa.or(() -> Optional.empty());
    var ob2 = ob.or(() -> Optional.empty());
    oa2.ifPresent(x2 -> x2.run());
    ob2.ifPresent(y2 -> y2.run());
    return 0;
  }

  public static int useOrVarOrElse(Optional<A> oa, Optional<B> ob, A da, B db) {
    var oa3 = oa.or(() -> Optional.of(da));
    var ob3 = ob.or(() -> Optional.of(db));
    var xa4 = oa3.orElse(da);
    var xb4 = ob3.orElse(db);
    return xa4.run() + xb4.run();
  }

  // Factory receiver: Optional.of(...).or(...).
  public static int useOfOrIfPresent() {
    Optional.of(new A()).or(() -> Optional.empty()).ifPresent(x3 -> x3.run());
    Optional.of(new B()).or(() -> Optional.empty()).ifPresent(y3 -> y3.run());
    return 0;
  }

  // Regression: bare Optional.ifPresent / orElse already worked.
  public static int useIfPresent(Optional<A> oa, Optional<B> ob) {
    oa.ifPresent(x4 -> x4.run());
    ob.ifPresent(y4 -> y4.run());
    return 0;
  }

  public static int useOrElse(Optional<A> oa, Optional<B> ob, A da, B db) {
    var xa5 = oa.orElse(da);
    var xb5 = ob.orElse(db);
    return xa5.run() + xb5.run();
  }

  // Regression: Optional.filter already peeled (sibling always-T stage).
  public static int useFilterIfPresent(Optional<A> oa, Optional<B> ob) {
    oa.filter(a5 -> true).ifPresent(x5 -> x5.run());
    ob.filter(b5 -> true).ifPresent(y5 -> y5.run());
    return 0;
  }
}
