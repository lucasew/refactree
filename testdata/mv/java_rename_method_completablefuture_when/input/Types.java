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
  // whenComplete BiConsumer body — T under foreign same-leaf.
  // Isolated: no same-name lambda params that file-scoped typedLocals would mask.
  public static int useWhenComplete(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    fa.whenComplete((a1, e1) -> {
      a1.run();
    });
    fb.whenComplete((b1, e2) -> {
      b1.run();
    });
    return 0;
  }

  // Expression-bodied whenComplete.
  public static int useWhenCompleteExpr(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    fa.whenComplete((a2, e3) -> a2.run());
    fb.whenComplete((b2, e4) -> b2.run());
    return 0;
  }

  // handle BiFunction body (return value unused for typing).
  public static int useHandleBody(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    fa.handle((a3, e5) -> {
      a3.run();
      return a3;
    });
    fb.handle((b3, e6) -> {
      b3.run();
      return b3;
    });
    return 0;
  }

  // Regression: thenAccept unary already worked.
  public static int useThenAccept(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    fa.thenAccept(a4 -> a4.run());
    fb.thenAccept(b4 -> b4.run());
    return 0;
  }
}
