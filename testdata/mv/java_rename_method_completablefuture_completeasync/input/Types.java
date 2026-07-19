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
  // completeAsync returns this CF; type-preserving by signature.
  public static int useCompleteAsyncJoin() {
    return new CompletableFuture<A>().completeAsync(() -> new A()).join().run()
        + new CompletableFuture<B>().completeAsync(() -> new B()).join().run();
  }

  public static int useCompleteAsyncExecutor() {
    Executor ex = ForkJoinPool.commonPool();
    return new CompletableFuture<A>().completeAsync(() -> new A(), ex).join().run()
        + new CompletableFuture<B>().completeAsync(() -> new B(), ex).join().run();
  }

  public static int useVarCompleteAsync() {
    var fa = new CompletableFuture<A>().completeAsync(() -> new A());
    var fb = new CompletableFuture<B>().completeAsync(() -> new B());
    return fa.join().run() + fb.join().run();
  }

  public static int usePreservesB() {
    return new CompletableFuture<B>().completeAsync(() -> new B()).join().run();
  }
}
