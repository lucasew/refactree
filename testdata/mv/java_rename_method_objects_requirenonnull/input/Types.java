package demo;

import java.util.List;
import java.util.Objects;

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
  public static int useRequireNonNull(A a, B b) {
    return Objects.requireNonNull(a).run() + Objects.requireNonNull(b).run();
  }

  public static int useRequireNonNullVar(A a, B b) {
    var xa = Objects.requireNonNull(a);
    var xb = Objects.requireNonNull(b);
    return xa.run() + xb.run();
  }

  public static int useRequireNonNullMsg(A a, B b) {
    return Objects.requireNonNull(a, "a").run() + Objects.requireNonNull(b, "b").run();
  }

  public static int useRequireNonNullMsgVar(A a, B b) {
    var xa = Objects.requireNonNull(a, "a");
    var xb = Objects.requireNonNull(b, "b");
    return xa.run() + xb.run();
  }

  public static int useRequireNonNullNew() {
    return Objects.requireNonNull(new A()).run() + Objects.requireNonNull(new B()).run();
  }

  public static int useRequireNonNullNewVar() {
    var xa = Objects.requireNonNull(new A());
    var xb = Objects.requireNonNull(new B());
    return xa.run() + xb.run();
  }

  public static int useRequireNonNullGet(List<A> as, List<B> bs) {
    return Objects.requireNonNull(as.get(0)).run() + Objects.requireNonNull(bs.get(0)).run();
  }

  public static int useRequireNonNullGetVar(List<A> as, List<B> bs) {
    var xa = Objects.requireNonNull(as.get(0));
    var xb = Objects.requireNonNull(bs.get(0));
    return xa.run() + xb.run();
  }

  public static int usePreservesB(B b) {
    return Objects.requireNonNull(b).run() + Objects.requireNonNull(b, "b").run();
  }
}
