package demo;

import java.util.List;
import java.util.Objects;

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
  public static int useElse(A a, A da, B b, B db) {
    return Objects.requireNonNullElse(a, da).execute() + Objects.requireNonNullElse(b, db).run();
  }

  public static int useElseVar(A a, A da, B b, B db) {
    var xa = Objects.requireNonNullElse(a, da);
    var xb = Objects.requireNonNullElse(b, db);
    return xa.execute() + xb.run();
  }

  public static int useElseGet(A a, B b) {
    return Objects.requireNonNullElseGet(a, () -> new A()).execute()
        + Objects.requireNonNullElseGet(b, () -> new B()).run();
  }

  public static int useElseGetVar(A a, B b) {
    var xa = Objects.requireNonNullElseGet(a, () -> new A());
    var xb = Objects.requireNonNullElseGet(b, () -> new B());
    return xa.execute() + xb.run();
  }

  public static int useElseNew() {
    return Objects.requireNonNullElse(new A(), new A()).execute()
        + Objects.requireNonNullElse(new B(), new B()).run();
  }

  public static int useElseGetList(List<A> as, List<B> bs) {
    return Objects.requireNonNullElseGet(as.get(0), () -> new A()).execute()
        + Objects.requireNonNullElseGet(bs.get(0), () -> new B()).run();
  }

  public static int usePreservesB(B b, B db) {
    return Objects.requireNonNullElse(b, db).run()
        + Objects.requireNonNullElseGet(b, () -> new B()).run();
  }
}
