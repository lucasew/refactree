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
  // Identity thenApply + join chain — T under foreign same-leaf.
  // Isolated: no same-name lambda params that file-scoped typedLocals would mask.
  public static int useThenApplyJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.thenApply(a1 -> a1).join().run() + fb.thenApply(b1 -> b1).join().run();
  }

  public static int useThenApplyJoinVar(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    var xa = fa.thenApply(a2 -> a2).join();
    var xb = fb.thenApply(b2 -> b2).join();
    return xa.run() + xb.run();
  }

  // Identity thenApply bound to var, then join.
  public static int useThenApplyVarJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    var fa2 = fa.thenApply(a3 -> a3);
    var fb2 = fb.thenApply(b3 -> b3);
    return fa2.join().run() + fb2.join().run();
  }

  // getNow / resultNow siblings of join on identity thenApply.
  public static int useThenApplyGetNow(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.thenApply(a4 -> a4).getNow(null).run() + fb.thenApply(b4 -> b4).getNow(null).run();
  }

  public static int useThenApplyResultNow(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.thenApply(a5 -> a5).resultNow().run() + fb.thenApply(b5 -> b5).resultNow().run();
  }

  // Regression: bare join already worked.
  public static int useJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.join().run() + fb.join().run();
  }

  // Regression: thenApply body already worked.
  public static int useThenApplyBody(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    fa.thenApply(a6 -> {
      a6.run();
      return a6;
    });
    fb.thenApply(b6 -> {
      b6.run();
      return b6;
    });
    return 0;
  }
}
