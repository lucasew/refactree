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
  public static int useThenComposeAsyncJoin() {
    return CompletableFuture.completedFuture(new A())
            .thenComposeAsync(a -> CompletableFuture.completedFuture(a))
            .join()
            .run()
        + CompletableFuture.completedFuture(new B())
            .thenComposeAsync(b -> CompletableFuture.completedFuture(b))
            .join()
            .run();
  }

  public static int useThenComposeAsyncExecutor() {
    Executor ex = ForkJoinPool.commonPool();
    return CompletableFuture.completedFuture(new A())
            .thenComposeAsync(a -> CompletableFuture.completedFuture(a), ex)
            .join()
            .run()
        + CompletableFuture.completedFuture(new B())
            .thenComposeAsync(b -> CompletableFuture.completedFuture(b), ex)
            .join()
            .run();
  }

  public static int useVarThenComposeAsync() {
    var fa = CompletableFuture.completedFuture(new A())
        .thenComposeAsync(a -> CompletableFuture.completedFuture(a));
    var fb = CompletableFuture.completedFuture(new B())
        .thenComposeAsync(b -> CompletableFuture.completedFuture(b));
    return fa.join().run() + fb.join().run();
  }

  public static int usePreservesB() {
    return CompletableFuture.completedFuture(new B())
        .thenComposeAsync(b -> CompletableFuture.completedFuture(b))
        .join()
        .run();
  }
}
