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
  // Diamond CF: T recovered from completeAsync supplier body.
  public static int useDiamondCompleteAsyncJoin() {
    return new CompletableFuture<>().completeAsync(() -> new A()).join().execute()
        + new CompletableFuture<>().completeAsync(() -> new B()).join().run();
  }

  public static int useDiamondCompleteAsyncExecutor() {
    Executor ex = ForkJoinPool.commonPool();
    return new CompletableFuture<>().completeAsync(() -> new A(), ex).join().execute()
        + new CompletableFuture<>().completeAsync(() -> new B(), ex).join().run();
  }

  public static int useVarDiamondCompleteAsync() {
    var fa = new CompletableFuture<>().completeAsync(() -> new A());
    var fb = new CompletableFuture<>().completeAsync(() -> new B());
    return fa.join().execute() + fb.join().run();
  }

  public static int usePreservesB() {
    return new CompletableFuture<>().completeAsync(() -> new B()).join().run();
  }
}
