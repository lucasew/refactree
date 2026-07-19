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
  public static int useThenCombineAsyncJoin() {
    return CompletableFuture.completedFuture(new A())
            .thenCombineAsync(CompletableFuture.completedFuture(new A()), (a, o) -> a)
            .join()
            .execute()
        + CompletableFuture.completedFuture(new B())
            .thenCombineAsync(CompletableFuture.completedFuture(new B()), (b, o) -> b)
            .join()
            .run();
  }

  public static int useThenCombineAsyncExecutor() {
    Executor ex = ForkJoinPool.commonPool();
    return CompletableFuture.completedFuture(new A())
            .thenCombineAsync(CompletableFuture.completedFuture(new A()), (a, o) -> a, ex)
            .join()
            .execute()
        + CompletableFuture.completedFuture(new B())
            .thenCombineAsync(CompletableFuture.completedFuture(new B()), (b, o) -> b, ex)
            .join()
            .run();
  }

  public static int useVarThenCombineAsync() {
    var fa = CompletableFuture.completedFuture(new A())
        .thenCombineAsync(CompletableFuture.completedFuture(new A()), (a, o) -> a);
    var fb = CompletableFuture.completedFuture(new B())
        .thenCombineAsync(CompletableFuture.completedFuture(new B()), (b, o) -> b);
    return fa.join().execute() + fb.join().run();
  }

  public static int usePreservesB() {
    return CompletableFuture.completedFuture(new B())
        .thenCombineAsync(CompletableFuture.completedFuture(new B()), (b, o) -> b)
        .join()
        .run();
  }
}
