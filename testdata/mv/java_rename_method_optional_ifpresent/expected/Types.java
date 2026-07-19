package demo;

import java.util.List;
import java.util.Optional;

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
  public static int useFindFirst(List<A> as, List<B> bs) {
    as.stream().findFirst().ifPresent(a -> a.execute());
    bs.stream().findFirst().ifPresent(b -> b.run());
    return 0;
  }

  public static int useFindAny(List<A> as, List<B> bs) {
    as.stream().findAny().ifPresent(a -> a.execute());
    bs.stream().findAny().ifPresent(b -> b.run());
    return 0;
  }

  public static int useOptionalOf() {
    Optional.of(new A()).ifPresent(a -> a.execute());
    Optional.of(new B()).ifPresent(b -> b.run());
    return 0;
  }

  public static int useOptionalOfNullable() {
    Optional.ofNullable(new A()).ifPresent(a -> a.execute());
    Optional.ofNullable(new B()).ifPresent(b -> b.run());
    return 0;
  }

  public static int useOptionalParam(Optional<A> oa, Optional<B> ob) {
    oa.ifPresent(a -> a.execute());
    ob.ifPresent(b -> b.run());
    return 0;
  }

  public static int useFindFirstFilter(List<A> as) {
    as.stream().filter(a -> a.execute() > 0).findFirst().ifPresent(a -> a.execute());
    return 0;
  }
}
