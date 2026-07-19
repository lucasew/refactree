package demo;

import java.util.concurrent.CompletableFuture;
import java.util.concurrent.Executor;
import java.util.concurrent.ForkJoinPool;

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
  public static int useThenApplyAsyncJoin() {
    return CompletableFuture.completedFuture(new A()).thenApplyAsync(a -> a).join().run()
        + CompletableFuture.completedFuture(new B()).thenApplyAsync(b -> b).join().run();
  }

  public static int useThenApplyAsyncExecutor() {
    Executor ex = ForkJoinPool.commonPool();
    return CompletableFuture.completedFuture(new A()).thenApplyAsync(a -> a, ex).join().run()
        + CompletableFuture.completedFuture(new B()).thenApplyAsync(b -> b, ex).join().run();
  }

  public static int useVarThenApplyAsync() {
    var fa = CompletableFuture.completedFuture(new A()).thenApplyAsync(a -> a);
    var fb = CompletableFuture.completedFuture(new B()).thenApplyAsync(b -> b);
    return fa.join().run() + fb.join().run();
  }

  public static int usePreservesB() {
    return CompletableFuture.completedFuture(new B()).thenApplyAsync(b -> b).join().run();
  }
}
