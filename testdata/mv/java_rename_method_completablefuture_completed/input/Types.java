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
  // completedFuture factory + join — T under foreign same-leaf.
  // Isolated: unique names so file-scoped typedLocals cannot mask.
  public static int useCompletedJoin() {
    return CompletableFuture.completedFuture(new A()).join().run()
        + CompletableFuture.completedFuture(new B()).join().run();
  }

  public static int useCompletedGet() throws Exception {
    return CompletableFuture.completedFuture(new A()).get().run()
        + CompletableFuture.completedFuture(new B()).get().run();
  }

  public static int useCompletedGetNow() {
    return CompletableFuture.completedFuture(new A()).getNow(null).run()
        + CompletableFuture.completedFuture(new B()).getNow(null).run();
  }

  public static int useCompletedResultNow() {
    return CompletableFuture.completedFuture(new A()).resultNow().run()
        + CompletableFuture.completedFuture(new B()).resultNow().run();
  }

  // Factory bound to var, then join / getNow.
  public static int useCompletedVarJoin() {
    var fa = CompletableFuture.completedFuture(new A());
    var fb = CompletableFuture.completedFuture(new B());
    return fa.join().run() + fb.join().run();
  }

  public static int useCompletedVarGetNow() {
    var fa2 = CompletableFuture.completedFuture(new A());
    var fb2 = CompletableFuture.completedFuture(new B());
    return fa2.getNow(null).run() + fb2.getNow(null).run();
  }

  // Identity thenApply after completedFuture factory + join.
  public static int useCompletedThenApplyJoin() {
    return CompletableFuture.completedFuture(new A()).thenApply(x1 -> x1).join().run()
        + CompletableFuture.completedFuture(new B()).thenApply(y1 -> y1).join().run();
  }

  // Regression: typed CF param join already worked.
  public static int useJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.join().run() + fb.join().run();
  }
}
