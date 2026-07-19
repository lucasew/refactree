package demo;

import java.util.concurrent.FutureTask;

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
  // new FutureTask<>(() -> new T()).get() — Callable ctor under foreign same-leaf.
  public static int useNewCallableGet() throws Exception {
    return new FutureTask<>(() -> new A()).get().run()
        + new FutureTask<>(() -> new B()).get().run();
  }

  // Runnable + result ctor: second arg is V.
  public static int useNewRunnableResultGet() throws Exception {
    return new FutureTask<>(() -> {}, new A()).get().run()
        + new FutureTask<>(() -> {}, new B()).get().run();
  }

  // Declared type arg FutureTask<T>.
  public static int useTypedFutureTask() throws Exception {
    return new FutureTask<A>(() -> new A()).get().run()
        + new FutureTask<B>(() -> new B()).get().run();
  }

  public static int useVarFutureTask() throws Exception {
    var fa = new FutureTask<>(() -> new A());
    var fb = new FutureTask<>(() -> new B());
    return fa.get().run() + fb.get().run();
  }

  public static int usePreservesB() throws Exception {
    return new FutureTask<>(() -> new B()).get().run()
        + new FutureTask<>(() -> {}, new B()).get().run();
  }
}
