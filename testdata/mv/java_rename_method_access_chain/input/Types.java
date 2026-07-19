package demo;

import java.util.List;
import java.util.Map;
import java.util.Optional;
import java.util.concurrent.Future;
import java.util.concurrent.atomic.AtomicReference;
import java.util.function.Supplier;

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
  public static int useSupplierGet(Supplier<A> sa, Supplier<B> sb) {
    return sa.get().run() + sb.get().run();
  }

  public static int useOptionalGet(Optional<A> oa, Optional<B> ob) {
    return oa.get().run() + ob.get().run();
  }

  public static int useOptionalOrElse(Optional<A> oa, Optional<B> ob) {
    return oa.orElse(null).run() + ob.orElse(null).run();
  }

  public static int useListGet(List<A> as, List<B> bs) {
    return as.get(0).run() + bs.get(0).run();
  }

  public static int useListOfGet() {
    return List.of(new A()).get(0).run() + List.of(new B()).get(0).run();
  }

  public static int useMapGet(Map<String, A> am, Map<String, B> bm) {
    return am.get("k").run() + bm.get("k").run();
  }

  public static int useMapOfGet() {
    return Map.of("k", new A()).get("k").run() + Map.of("k", new B()).get("k").run();
  }

  public static int useAtomicGet(AtomicReference<A> aa, AtomicReference<B> ab) {
    return aa.get().run() + ab.get().run();
  }

  public static int useFutureGet(Future<A> fa, Future<B> fb) throws Exception {
    return fa.get().run() + fb.get().run();
  }

  public static int useFindFirstGet(List<A> as, List<B> bs) {
    return as.stream().findFirst().get().run() + bs.stream().findFirst().get().run();
  }

  public static int usePreservesB(Supplier<B> sb, Optional<B> ob, List<B> bs) {
    return sb.get().run() + ob.get().run() + bs.get(0).run();
  }
}
