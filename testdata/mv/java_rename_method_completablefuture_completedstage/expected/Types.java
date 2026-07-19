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
  // completedStage → toCompletableFuture → join under foreign same-leaf.
  public static int useCompletedStageJoin() {
    return CompletableFuture.completedStage(new A()).toCompletableFuture().join().execute()
        + CompletableFuture.completedStage(new B()).toCompletableFuture().join().run();
  }

  public static int useCompletedStageGet() throws Exception {
    return CompletableFuture.completedStage(new A()).toCompletableFuture().get().execute()
        + CompletableFuture.completedStage(new B()).toCompletableFuture().get().run();
  }

  public static int useVarCompletedStage() {
    var fa = CompletableFuture.completedStage(new A());
    var fb = CompletableFuture.completedStage(new B());
    return fa.toCompletableFuture().join().execute() + fb.toCompletableFuture().join().run();
  }

  public static int usePreservesB() {
    return CompletableFuture.completedStage(new B()).toCompletableFuture().join().run();
  }
}
