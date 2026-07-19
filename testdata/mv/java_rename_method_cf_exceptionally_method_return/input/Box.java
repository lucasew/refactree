import java.util.concurrent.CompletableFuture;

class A {
  int run() {
    return 1;
  }
}

class B {
  int run() {
    return 2;
  }
}

class BoxA {
  A a = new A();

  A get() {
    return a;
  }
}

class BoxB {
  B b = new B();

  B get() {
    return b;
  }
}

class Use {
  // Class: failedFuture.exceptionally(t -> new T()) — was UNDER.
  int useClassFailed() {
    return CompletableFuture.failedFuture(new RuntimeException()).exceptionally(t -> new A()).join().run()
        + CompletableFuture.failedFuture(new RuntimeException()).exceptionally(t -> new B()).join().run();
  }

  int useClassFailedAssign() {
    A csa = CompletableFuture.failedFuture(new RuntimeException()).exceptionally(t -> new A()).join();
    B csb = CompletableFuture.failedFuture(new RuntimeException()).exceptionally(t -> new B()).join();
    return csa.run() + csb.run();
  }

  // Method-return: failedFuture.exceptionally(t -> ba.get()) — was UNDER.
  int useMRFailed(BoxA ba, BoxB bb) {
    return CompletableFuture.failedFuture(new RuntimeException()).exceptionally(t -> ba.get()).join().run()
        + CompletableFuture.failedFuture(new RuntimeException()).exceptionally(t -> bb.get()).join().run();
  }

  int useMRFailedAssign(BoxA ba, BoxB bb) {
    A msa = CompletableFuture.failedFuture(new RuntimeException()).exceptionally(t -> ba.get()).join();
    B msb = CompletableFuture.failedFuture(new RuntimeException()).exceptionally(t -> bb.get()).join();
    return msa.run() + msb.run();
  }

  // Preserves B under foreign same-leaf.
  int usePreservesB(BoxB bb) {
    return CompletableFuture.failedFuture(new RuntimeException()).exceptionally(t -> bb.get()).join().run()
        + CompletableFuture.failedFuture(new RuntimeException()).exceptionally(t -> new B()).join().run();
  }
}
