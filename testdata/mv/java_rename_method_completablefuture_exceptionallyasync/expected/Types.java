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
  // exceptionallyAsync is type-preserving by signature (recovery yields T).
  public static int useExceptionallyAsyncJoin() {
    return CompletableFuture.completedFuture(new A())
            .exceptionallyAsync(ex -> new A())
            .join()
            .execute()
        + CompletableFuture.completedFuture(new B())
            .exceptionallyAsync(ex -> new B())
            .join()
            .run();
  }

  public static int useExceptionallyAsyncExecutor() {
    Executor ex = ForkJoinPool.commonPool();
    return CompletableFuture.completedFuture(new A())
            .exceptionallyAsync(ex2 -> new A(), ex)
            .join()
            .execute()
        + CompletableFuture.completedFuture(new B())
            .exceptionallyAsync(ex2 -> new B(), ex)
            .join()
            .run();
  }

  public static int useVarExceptionallyAsync() {
    var fa = CompletableFuture.completedFuture(new A()).exceptionallyAsync(ex -> new A());
    var fb = CompletableFuture.completedFuture(new B()).exceptionallyAsync(ex -> new B());
    return fa.join().execute() + fb.join().run();
  }

  public static int usePreservesB() {
    return CompletableFuture.completedFuture(new B())
        .exceptionallyAsync(ex -> new B())
        .join()
        .run();
  }
}
