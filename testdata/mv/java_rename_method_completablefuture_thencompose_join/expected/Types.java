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
  // completedFuture rewrap thenCompose + join — T under foreign same-leaf.
  // Isolated: no same-name lambda params that file-scoped typedLocals would mask.
  public static int useThenComposeJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.thenCompose(a1 -> CompletableFuture.completedFuture(a1)).join().execute()
        + fb.thenCompose(b1 -> CompletableFuture.completedFuture(b1)).join().run();
  }

  public static int useThenComposeJoinVar(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    var xa = fa.thenCompose(a2 -> CompletableFuture.completedFuture(a2)).join();
    var xb = fb.thenCompose(b2 -> CompletableFuture.completedFuture(b2)).join();
    return xa.execute() + xb.run();
  }

  // completedFuture rewrap bound to var, then join.
  public static int useThenComposeVarJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    var fa2 = fa.thenCompose(a3 -> CompletableFuture.completedFuture(a3));
    var fb2 = fb.thenCompose(b3 -> CompletableFuture.completedFuture(b3));
    return fa2.join().execute() + fb2.join().run();
  }

  // getNow / resultNow siblings of join on thenCompose rewrap.
  public static int useThenComposeGetNow(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.thenCompose(a4 -> CompletableFuture.completedFuture(a4)).getNow(null).execute()
        + fb.thenCompose(b4 -> CompletableFuture.completedFuture(b4)).getNow(null).run();
  }

  public static int useThenComposeResultNow(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.thenCompose(a5 -> CompletableFuture.completedFuture(a5)).resultNow().execute()
        + fb.thenCompose(b5 -> CompletableFuture.completedFuture(b5)).resultNow().run();
  }

  // Method-ref rewrap (same as Optional::of for flatMap).
  public static int useThenComposeMethodRef(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.thenCompose(CompletableFuture::completedFuture).join().execute()
        + fb.thenCompose(CompletableFuture::completedFuture).join().run();
  }

  // new T() rewrap also peels (same as Optional.of(new A()) flatMap).
  public static int useThenComposeNew(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.thenCompose(a6 -> CompletableFuture.completedFuture(new A())).join().execute()
        + fb.thenCompose(b6 -> CompletableFuture.completedFuture(new B())).join().run();
  }

  // Regression: bare join already worked.
  public static int useJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.join().execute() + fb.join().run();
  }

  // Regression: thenApply identity join already worked.
  public static int useThenApplyJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.thenApply(a7 -> a7).join().execute() + fb.thenApply(b7 -> b7).join().run();
  }

  // Regression: thenCompose body already worked.
  public static int useThenComposeBody(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    fa.thenCompose(a8 -> {
      a8.execute();
      return CompletableFuture.completedFuture(a8);
    });
    fb.thenCompose(b8 -> {
      b8.run();
      return CompletableFuture.completedFuture(b8);
    });
    return 0;
  }
}
