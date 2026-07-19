import java.util.concurrent.CompletableFuture;
import java.util.concurrent.Executor;
import java.util.concurrent.ForkJoinPool;

class A {
  int execute() {
    return 1;
  }
}

class B {
  int run() {
    return 2;
  }
}

class BoxA {
  A a;

  BoxA(A a) {
    this.a = a;
  }

  A get() {
    return a;
  }
}

class BoxB {
  B b;

  BoxB(B b) {
    this.b = b;
  }

  B get() {
    return b;
  }
}

class Use {
  // Diamond completeAsync: T from supplier method-return under foreign same-leaf.
  int useDiamondJoin(BoxA ba, BoxB bb) {
    return new CompletableFuture<>().completeAsync(() -> ba.get()).join().execute()
        + new CompletableFuture<>().completeAsync(() -> bb.get()).join().run();
  }

  int useDiamondExecutor(BoxA ba, BoxB bb) {
    Executor ex = ForkJoinPool.commonPool();
    return new CompletableFuture<>().completeAsync(() -> ba.get(), ex).join().execute()
        + new CompletableFuture<>().completeAsync(() -> bb.get(), ex).join().run();
  }

  int useDiamondVar(BoxA ba, BoxB bb) {
    var fa = new CompletableFuture<>().completeAsync(() -> ba.get());
    var fb = new CompletableFuture<>().completeAsync(() -> bb.get());
    return fa.join().execute() + fb.join().run();
  }

  int useDiamondThenApply(BoxA ba, BoxB bb) {
    return new CompletableFuture<>().completeAsync(() -> ba.get()).thenApply(x -> x).join().execute()
        + new CompletableFuture<>().completeAsync(() -> bb.get()).thenApply(x -> x).join().run();
  }

  // Class regression — diamond supplier new T() already worked.
  int useDiamondJoinClass() {
    return new CompletableFuture<>().completeAsync(() -> new A()).join().execute()
        + new CompletableFuture<>().completeAsync(() -> new B()).join().run();
  }

  int useDiamondVarClass() {
    var fa = new CompletableFuture<>().completeAsync(() -> new A());
    var fb = new CompletableFuture<>().completeAsync(() -> new B());
    return fa.join().execute() + fb.join().run();
  }

  // Typed CF + method-return already solid; keep as neighbor regression.
  int useTypedJoin(BoxA ba, BoxB bb) {
    return new CompletableFuture<A>().completeAsync(() -> ba.get()).join().execute()
        + new CompletableFuture<B>().completeAsync(() -> bb.get()).join().run();
  }

  int usePreservesB(BoxB bb) {
    return new CompletableFuture<>().completeAsync(() -> bb.get()).join().run()
        + new CompletableFuture<>().completeAsync(() -> new B()).join().run();
  }
}
