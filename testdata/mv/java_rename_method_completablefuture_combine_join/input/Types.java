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
  // Identity thenCombine (bi-lambda first-param return) + join — T under foreign same-leaf.
  // Isolated: no same-name lambda params that file-scoped typedLocals would mask.
  public static int useThenCombineJoin(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    return fa.thenCombine(fa2, (a1, a2) -> a1).join().run()
        + fb.thenCombine(fb2, (b1, b2) -> b1).join().run();
  }

  public static int useThenCombineJoinVar(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    var xa = fa.thenCombine(fa2, (a3, a4) -> a3).join();
    var xb = fb.thenCombine(fb2, (b3, b4) -> b3).join();
    return xa.run() + xb.run();
  }

  // Identity thenCombine bound to var, then join.
  public static int useThenCombineVarJoin(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    var fa3 = fa.thenCombine(fa2, (a5, a6) -> a5);
    var fb3 = fb.thenCombine(fb2, (b5, b6) -> b5);
    return fa3.join().run() + fb3.join().run();
  }

  // getNow / resultNow siblings of join on identity thenCombine.
  public static int useThenCombineGetNow(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    return fa.thenCombine(fa2, (a7, a8) -> a7).getNow(null).run()
        + fb.thenCombine(fb2, (b7, b8) -> b7).getNow(null).run();
  }

  public static int useThenCombineResultNow(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    return fa.thenCombine(fa2, (a9, a10) -> a9).resultNow().run()
        + fb.thenCombine(fb2, (b9, b10) -> b9).resultNow().run();
  }

  // new T() BiFunction also peels (same as handle / applyToEither).
  public static int useThenCombineNew(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    return fa.thenCombine(fa2, (a11, a12) -> new A()).join().run()
        + fb.thenCombine(fb2, (b11, b12) -> new B()).join().run();
  }

  // Regression: bare join already worked.
  public static int useJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.join().run() + fb.join().run();
  }

  // Regression: thenApply identity join already worked.
  public static int useThenApplyJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.thenApply(a13 -> a13).join().run() + fb.thenApply(b13 -> b13).join().run();
  }

  // Regression: thenCombine bi-lambda body already worked.
  public static int useThenCombineBody(CompletableFuture<A> fa, CompletableFuture<A> fa2,
      CompletableFuture<B> fb, CompletableFuture<B> fb2) {
    fa.thenCombine(fa2, (a14, a15) -> {
      a14.run();
      return a14;
    });
    fb.thenCombine(fb2, (b14, b15) -> {
      b14.run();
      return b14;
    });
    return 0;
  }
}
