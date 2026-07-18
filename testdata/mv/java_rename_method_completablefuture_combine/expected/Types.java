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
  // thenCombine bi-lambda — both params T when other is same CF type.
  // Isolated names: no same-name lambda params that file-scoped typedLocals would mask.
  public static int useThenCombine(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    fa.thenCombine(fa, (a1, a2) -> {
      a1.execute();
      return a1;
    });
    fb.thenCombine(fb, (b1, b2) -> {
      b1.run();
      return b1;
    });
    return 0;
  }

  public static int useThenAcceptBoth(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    fa.thenAcceptBoth(fa, (a3, a4) -> a3.execute());
    fb.thenAcceptBoth(fb, (b3, b4) -> b3.run());
    return 0;
  }

  // Second param from other stage of same type.
  public static int useThenCombineOther(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    fa.thenCombine(fa2, (a7, a8) -> {
      a7.execute();
      a8.execute();
      return a7;
    });
    fb.thenCombine(fb2, (b7, b8) -> {
      b7.run();
      b8.run();
      return b7;
    });
    return 0;
  }

  // Regression: thenAccept unary already worked.
  public static int useThenAccept(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    fa.thenAccept(a9 -> a9.execute());
    fb.thenAccept(b9 -> b9.run());
    return 0;
  }
}
