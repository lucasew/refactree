package demo;

import java.util.List;
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
  public static int useFindFirst(List<A> as, List<B> bs) {
    as.stream().findFirst().ifPresentOrElse(a -> a.run(), () -> {});
    bs.stream().findFirst().ifPresentOrElse(b -> b.run(), () -> {});
    return 0;
  }

  public static int useFindAny(List<A> as, List<B> bs) {
    as.stream().findAny().ifPresentOrElse(a -> a.run(), () -> {});
    bs.stream().findAny().ifPresentOrElse(b -> b.run(), () -> {});
    return 0;
  }

  public static int useOptionalOf() {
    Optional.of(new A()).ifPresentOrElse(a -> a.run(), () -> {});
    Optional.of(new B()).ifPresentOrElse(b -> b.run(), () -> {});
    return 0;
  }

  public static int useOptionalOfNullable() {
    Optional.ofNullable(new A()).ifPresentOrElse(a -> a.run(), () -> {});
    Optional.ofNullable(new B()).ifPresentOrElse(b -> b.run(), () -> {});
    return 0;
  }

  public static int useOptionalParam(Optional<A> oa, Optional<B> ob) {
    oa.ifPresentOrElse(a -> a.run(), () -> {});
    ob.ifPresentOrElse(b -> b.run(), () -> {});
    return 0;
  }

  public static int useFindFirstFilter(List<A> as) {
    as.stream().filter(a -> a.run() > 0).findFirst().ifPresentOrElse(a -> a.run(), () -> {});
    return 0;
  }
}
