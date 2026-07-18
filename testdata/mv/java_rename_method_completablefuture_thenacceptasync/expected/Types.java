package demo;

import java.util.concurrent.CompletableFuture;
import java.util.concurrent.Executor;
import java.util.concurrent.ForkJoinPool;

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
  public static int useThenAcceptAsync() {
    CompletableFuture.completedFuture(new A()).thenAcceptAsync(a -> a.execute());
    CompletableFuture.completedFuture(new B()).thenAcceptAsync(b -> b.run());
    return 0;
  }

  public static int useThenAcceptAsyncExecutor() {
    Executor ex = ForkJoinPool.commonPool();
    CompletableFuture.completedFuture(new A()).thenAcceptAsync(a -> a.execute(), ex);
    CompletableFuture.completedFuture(new B()).thenAcceptAsync(b -> b.run(), ex);
    return 0;
  }

  public static int usePreservesB() {
    CompletableFuture.completedFuture(new B()).thenAcceptAsync(b -> b.run());
    return 0;
  }
}
