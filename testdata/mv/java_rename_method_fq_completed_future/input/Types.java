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
  public static int useFQCompleted() {
    return java.util.concurrent.CompletableFuture.completedFuture(new A()).join().run()
        + java.util.concurrent.CompletableFuture.completedFuture(new B()).join().run();
  }

  public static int useFQCompletedStage() {
    return java.util.concurrent.CompletableFuture.completedStage(new A())
            .toCompletableFuture()
            .join()
            .run()
        + java.util.concurrent.CompletableFuture.completedStage(new B())
            .toCompletableFuture()
            .join()
            .run();
  }

  public static int useFQVar() {
    var fa = java.util.concurrent.CompletableFuture.completedFuture(new A());
    var fb = java.util.concurrent.CompletableFuture.completedFuture(new B());
    return fa.join().run() + fb.join().run();
  }

  public static int usePreservesB() {
    return java.util.concurrent.CompletableFuture.completedFuture(new B()).join().run();
  }
}
