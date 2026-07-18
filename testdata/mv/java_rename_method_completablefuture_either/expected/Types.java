package demo;

import java.util.concurrent.CompletableFuture;

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
  // applyToEither Function body — T under foreign same-leaf.
  // Isolated names: no same-name lambda params that file-scoped typedLocals would mask.
  public static int useApplyToEither(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    fa.applyToEither(fa2, a1 -> {
      a1.execute();
      return a1;
    });
    fb.applyToEither(fb2, b1 -> {
      b1.run();
      return b1;
    });
    return 0;
  }

  // Expression-bodied applyToEither.
  public static int useApplyToEitherExpr(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    fa.applyToEither(fa2, a2 -> a2.execute());
    fb.applyToEither(fb2, b2 -> b2.run());
    return 0;
  }

  public static int useAcceptEither(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    fa.acceptEither(fa2, a3 -> a3.execute());
    fb.acceptEither(fb2, b3 -> b3.run());
    return 0;
  }

  // Regression: thenAccept unary already worked.
  public static int useThenAccept(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    fa.thenAccept(a9 -> a9.execute());
    fb.thenAccept(b9 -> b9.run());
    return 0;
  }
}
