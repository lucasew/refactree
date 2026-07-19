package demo;

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
  public static int useWithInitialGet() {
    return ThreadLocal.withInitial(() -> new A()).get().run()
        + ThreadLocal.withInitial(() -> new B()).get().run();
  }

  public static int useVarWithInitial() {
    var ta = ThreadLocal.withInitial(() -> new A());
    var tb = ThreadLocal.withInitial(() -> new B());
    return ta.get().run() + tb.get().run();
  }

  public static int usePreservesB() {
    return ThreadLocal.withInitial(() -> new B()).get().run();
  }
}
