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
  public static int useOrElseThrow(Optional<A> oa, Optional<B> ob) {
    var xa = oa.orElseThrow();
    var xb = ob.orElseThrow();
    return xa.run() + xb.run();
  }

  public static int useOrElseThrowSupplier(Optional<A> oa, Optional<B> ob) {
    var xa = oa.orElseThrow(() -> new RuntimeException());
    var xb = ob.orElseThrow(() -> new RuntimeException());
    return xa.run() + xb.run();
  }

  public static int useFindFirstOrElseThrow(List<A> as, List<B> bs) {
    var xa = as.stream().findFirst().orElseThrow();
    var xb = bs.stream().findFirst().orElseThrow();
    return xa.run() + xb.run();
  }

  public static int useOptionalOfOrElseThrow() {
    var xa = Optional.of(new A()).orElseThrow();
    var xb = Optional.of(new B()).orElseThrow();
    return xa.run() + xb.run();
  }
}
