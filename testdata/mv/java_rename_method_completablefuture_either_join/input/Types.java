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
  // Identity applyToEither + join chain — T under foreign same-leaf.
  // Isolated: no same-name lambda params that file-scoped typedLocals would mask.
  public static int useEitherJoin(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    return fa.applyToEither(fa2, a1 -> a1).join().run()
        + fb.applyToEither(fb2, b1 -> b1).join().run();
  }

  public static int useEitherJoinVar(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    var xa = fa.applyToEither(fa2, a2 -> a2).join();
    var xb = fb.applyToEither(fb2, b2 -> b2).join();
    return xa.run() + xb.run();
  }

  // Identity applyToEither bound to var, then join.
  public static int useEitherVarJoin(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    var fa3 = fa.applyToEither(fa2, a3 -> a3);
    var fb3 = fb.applyToEither(fb2, b3 -> b3);
    return fa3.join().run() + fb3.join().run();
  }

  // getNow / resultNow siblings of join on identity applyToEither.
  public static int useEitherGetNow(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    return fa.applyToEither(fa2, a4 -> a4).getNow(null).run()
        + fb.applyToEither(fb2, b4 -> b4).getNow(null).run();
  }

  public static int useEitherResultNow(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    return fa.applyToEither(fa2, a5 -> a5).resultNow().run()
        + fb.applyToEither(fb2, b5 -> b5).resultNow().run();
  }

  // new T() mapper also peels (same as thenApply).
  public static int useEitherNew(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    return fa.applyToEither(fa2, a6 -> new A()).join().run()
        + fb.applyToEither(fb2, b6 -> new B()).join().run();
  }

  // Regression: bare join already worked.
  public static int useJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.join().run() + fb.join().run();
  }

  // Regression: thenApply identity join already worked.
  public static int useThenApplyJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.thenApply(a9 -> a9).join().run() + fb.thenApply(b9 -> b9).join().run();
  }

  // Regression: applyToEither Function body already worked.
  public static int useEitherBody(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    fa.applyToEither(fa2, a8 -> {
      a8.run();
      return a8;
    });
    fb.applyToEither(fb2, b8 -> {
      b8.run();
      return b8;
    });
    return 0;
  }
}
