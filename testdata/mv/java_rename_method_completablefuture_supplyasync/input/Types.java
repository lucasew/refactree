package demo;

import java.util.concurrent.CompletableFuture;
import java.util.concurrent.Executor;

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
  // supplyAsync supplier body + join — T under foreign same-leaf.
  // Isolated: unique names so file-scoped typedLocals cannot mask.
  public static int useSupplyJoin() {
    return CompletableFuture.supplyAsync(() -> new A()).join().run()
        + CompletableFuture.supplyAsync(() -> new B()).join().run();
  }

  public static int useSupplyGet() throws Exception {
    return CompletableFuture.supplyAsync(() -> new A()).get().run()
        + CompletableFuture.supplyAsync(() -> new B()).get().run();
  }

  public static int useSupplyGetNow() {
    return CompletableFuture.supplyAsync(() -> new A()).getNow(null).run()
        + CompletableFuture.supplyAsync(() -> new B()).getNow(null).run();
  }

  public static int useSupplyResultNow() {
    return CompletableFuture.supplyAsync(() -> new A()).resultNow().run()
        + CompletableFuture.supplyAsync(() -> new B()).resultNow().run();
  }

  // Optional Executor overload — executor ignored for type peel.
  public static int useSupplyExecJoin(Executor ex) {
    return CompletableFuture.supplyAsync(() -> new A(), ex).join().run()
        + CompletableFuture.supplyAsync(() -> new B(), ex).join().run();
  }

  // Factory bound to var, then join / getNow.
  public static int useSupplyVarJoin() {
    var fa = CompletableFuture.supplyAsync(() -> new A());
    var fb = CompletableFuture.supplyAsync(() -> new B());
    return fa.join().run() + fb.join().run();
  }

  public static int useSupplyVarGetNow() {
    var fa2 = CompletableFuture.supplyAsync(() -> new A());
    var fb2 = CompletableFuture.supplyAsync(() -> new B());
    return fa2.getNow(null).run() + fb2.getNow(null).run();
  }

  // Identity thenApply after supplyAsync factory + join.
  public static int useSupplyThenApplyJoin() {
    return CompletableFuture.supplyAsync(() -> new A()).thenApply(x1 -> x1).join().run()
        + CompletableFuture.supplyAsync(() -> new B()).thenApply(y1 -> y1).join().run();
  }

  // Regression: typed CF param join already worked.
  public static int useJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.join().run() + fb.join().run();
  }
}
