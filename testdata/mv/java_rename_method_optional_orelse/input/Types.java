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
  public static int useOrElse(Optional<A> oa, Optional<B> ob, A da, B db) {
    var xa = oa.orElse(da);
    var xb = ob.orElse(db);
    return xa.run() + xb.run();
  }

  public static int useOrElseGet(Optional<A> oa, Optional<B> ob) {
    var xa = oa.orElseGet(() -> new A());
    var xb = ob.orElseGet(() -> new B());
    return xa.run() + xb.run();
  }

  public static int useFindFirstOrElse(List<A> as, List<B> bs, A da, B db) {
    var xa = as.stream().findFirst().orElse(da);
    var xb = bs.stream().findFirst().orElse(db);
    return xa.run() + xb.run();
  }

  public static int useOptionalOfOrElse(A da, B db) {
    var xa = Optional.of(new A()).orElse(da);
    var xb = Optional.of(new B()).orElse(db);
    return xa.run() + xb.run();
  }
}
