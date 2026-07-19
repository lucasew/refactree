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
  public static int useApplyToEitherAsyncJoin() {
    return CompletableFuture.completedFuture(new A())
            .applyToEitherAsync(CompletableFuture.completedFuture(new A()), a -> a)
            .join()
            .execute()
        + CompletableFuture.completedFuture(new B())
            .applyToEitherAsync(CompletableFuture.completedFuture(new B()), b -> b)
            .join()
            .run();
  }

  public static int useApplyToEitherAsyncExecutor() {
    Executor ex = ForkJoinPool.commonPool();
    return CompletableFuture.completedFuture(new A())
            .applyToEitherAsync(CompletableFuture.completedFuture(new A()), a -> a, ex)
            .join()
            .execute()
        + CompletableFuture.completedFuture(new B())
            .applyToEitherAsync(CompletableFuture.completedFuture(new B()), b -> b, ex)
            .join()
            .run();
  }

  public static int useVarApplyToEitherAsync() {
    var fa = CompletableFuture.completedFuture(new A())
        .applyToEitherAsync(CompletableFuture.completedFuture(new A()), a -> a);
    var fb = CompletableFuture.completedFuture(new B())
        .applyToEitherAsync(CompletableFuture.completedFuture(new B()), b -> b);
    return fa.join().execute() + fb.join().run();
  }

  public static int usePreservesB() {
    return CompletableFuture.completedFuture(new B())
        .applyToEitherAsync(CompletableFuture.completedFuture(new B()), b -> b)
        .join()
        .run();
  }
}
