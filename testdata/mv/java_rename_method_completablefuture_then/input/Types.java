package demo;

import java.util.concurrent.CompletableFuture;

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
  // thenAccept Consumer body — T under foreign same-leaf.
  // Isolated: no same-name lambda params that file-scoped typedLocals would mask.
  public static int useThenAccept(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    fa.thenAccept(a1 -> {
      a1.run();
    });
    fb.thenAccept(b1 -> {
      b1.run();
    });
    return 0;
  }

  // Expression-bodied thenAccept.
  public static int useThenAcceptExpr(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    fa.thenAccept(a2 -> a2.run());
    fb.thenAccept(b2 -> b2.run());
    return 0;
  }

  // thenApply Function body (return value unused for typing).
  public static int useThenApplyBody(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    fa.thenApply(a3 -> {
      a3.run();
      return a3;
    });
    fb.thenApply(b3 -> {
      b3.run();
      return b3;
    });
    return 0;
  }

  // thenCompose Function body.
  public static int useThenComposeBody(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    fa.thenCompose(a4 -> {
      a4.run();
      return CompletableFuture.completedFuture(a4);
    });
    fb.thenCompose(b4 -> {
      b4.run();
      return CompletableFuture.completedFuture(b4);
    });
    return 0;
  }

  // Regression: join / getNow / resultNow already worked.
  public static int useJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.join().run() + fb.join().run();
  }
}
