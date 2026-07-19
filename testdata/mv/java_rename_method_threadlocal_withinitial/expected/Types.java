package demo;

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
  public static int useWithInitialGet() {
    return ThreadLocal.withInitial(() -> new A()).get().execute()
        + ThreadLocal.withInitial(() -> new B()).get().run();
  }

  public static int useVarWithInitial() {
    var ta = ThreadLocal.withInitial(() -> new A());
    var tb = ThreadLocal.withInitial(() -> new B());
    return ta.get().execute() + tb.get().run();
  }

  public static int usePreservesB() {
    return ThreadLocal.withInitial(() -> new B()).get().run();
  }
}
