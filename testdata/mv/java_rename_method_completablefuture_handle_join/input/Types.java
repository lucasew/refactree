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
  // Identity handle (bi-lambda first-param return) + join — T under foreign same-leaf.
  // Isolated: no same-name lambda params that file-scoped typedLocals would mask.
  public static int useHandleJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.handle((a1, e1) -> a1).join().run() + fb.handle((b1, e1) -> b1).join().run();
  }

  public static int useHandleJoinVar(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    var xa = fa.handle((a2, e2) -> a2).join();
    var xb = fb.handle((b2, e2) -> b2).join();
    return xa.run() + xb.run();
  }

  // Identity handle bound to var, then join.
  public static int useHandleVarJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    var fa2 = fa.handle((a3, e3) -> a3);
    var fb2 = fb.handle((b3, e3) -> b3);
    return fa2.join().run() + fb2.join().run();
  }

  // getNow / resultNow siblings of join on identity handle.
  public static int useHandleGetNow(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.handle((a4, e4) -> a4).getNow(null).run() + fb.handle((b4, e4) -> b4).getNow(null).run();
  }

  public static int useHandleResultNow(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.handle((a5, e5) -> a5).resultNow().run() + fb.handle((b5, e5) -> b5).resultNow().run();
  }

  // Regression: bare join already worked.
  public static int useJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.join().run() + fb.join().run();
  }

  // Regression: handle body first param already worked.
  public static int useHandleBody(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    fa.handle((a6, e6) -> {
      a6.run();
      return a6;
    });
    fb.handle((b6, e6) -> {
      b6.run();
      return b6;
    });
    return 0;
  }
}
