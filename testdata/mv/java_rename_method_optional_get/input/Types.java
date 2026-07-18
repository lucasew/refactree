package demo;

import java.util.List;
import java.util.Optional;
import java.util.stream.Stream;

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
  public static int useFindFirstGet(Stream<A> as, Stream<B> bs) {
    var xa = as.findFirst().get();
    var xb = bs.findFirst().get();
    return xa.run() + xb.run();
  }

  public static int useFindAnyGet(Stream<A> as, Stream<B> bs) {
    var xa = as.findAny().get();
    var xb = bs.findAny().get();
    return xa.run() + xb.run();
  }

  public static int useListStreamFindFirstGet(List<A> as, List<B> bs) {
    var xa = as.stream().findFirst().get();
    var xb = bs.stream().findFirst().get();
    return xa.run() + xb.run();
  }

  public static int useOptionalOfGet() {
    var xa = Optional.of(new A()).get();
    var xb = Optional.of(new B()).get();
    return xa.run() + xb.run();
  }

  public static int useOptionalOfNullableGet(A da, B db) {
    var xa = Optional.ofNullable(da).get();
    var xb = Optional.ofNullable(db).get();
    return xa.run() + xb.run();
  }

  public static int useOptionalLocalGet(Optional<A> oa, Optional<B> ob) {
    var xa = oa.get();
    var xb = ob.get();
    return xa.run() + xb.run();
  }

  public static int useAssignedFindFirstGet(List<A> as, List<B> bs) {
    var oa = as.stream().findFirst();
    var ob = bs.stream().findFirst();
    var xa = oa.get();
    var xb = ob.get();
    return xa.run() + xb.run();
  }

  public static int usePreservesB(Stream<B> bs) {
    var xb = bs.findFirst().get();
    return xb.run();
  }
}
