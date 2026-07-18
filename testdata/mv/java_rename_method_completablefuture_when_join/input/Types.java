package demo;

import java.util.concurrent.CompletableFuture;
import java.util.concurrent.TimeUnit;

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
  // whenComplete always preserves T + join — under foreign same-leaf.
  // Isolated: no same-name lambda params that file-scoped typedLocals would mask.
  public static int useWhenCompleteJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.whenComplete((a1, e1) -> {}).join().run()
        + fb.whenComplete((b1, e1) -> {}).join().run();
  }

  public static int useWhenCompleteJoinVar(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    var xa = fa.whenComplete((a2, e2) -> {}).join();
    var xb = fb.whenComplete((b2, e2) -> {}).join();
    return xa.run() + xb.run();
  }

  // whenComplete bound to var, then join.
  public static int useWhenCompleteVarJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    var fa2 = fa.whenComplete((a3, e3) -> {});
    var fb2 = fb.whenComplete((b3, e3) -> {});
    return fa2.join().run() + fb2.join().run();
  }

  // getNow / resultNow siblings of join on whenComplete.
  public static int useWhenCompleteGetNow(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.whenComplete((a4, e4) -> {}).getNow(null).run()
        + fb.whenComplete((b4, e4) -> {}).getNow(null).run();
  }

  public static int useWhenCompleteResultNow(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.whenComplete((a5, e5) -> {}).resultNow().run()
        + fb.whenComplete((b5, e5) -> {}).resultNow().run();
  }

  // Sibling always-T peels (same signature path as whenComplete).
  public static int useCopyJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.copy().join().run() + fb.copy().join().run();
  }

  public static int useToCFJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.toCompletableFuture().join().run() + fb.toCompletableFuture().join().run();
  }

  public static int useOrTimeoutJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.orTimeout(1, TimeUnit.SECONDS).join().run()
        + fb.orTimeout(1, TimeUnit.SECONDS).join().run();
  }

  public static int useExceptionallyJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.exceptionally(e -> new A()).join().run()
        + fb.exceptionally(e -> new B()).join().run();
  }

  // Regression: bare join already worked.
  public static int useJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.join().run() + fb.join().run();
  }

  // Regression: whenComplete body first param already worked.
  public static int useWhenCompleteBody(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    fa.whenComplete((a6, e6) -> {
      a6.run();
    });
    fb.whenComplete((b6, e6) -> {
      b6.run();
    });
    return 0;
  }
}
