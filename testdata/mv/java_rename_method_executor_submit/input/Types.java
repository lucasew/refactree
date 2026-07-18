package demo;

import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

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
  public static int useSubmitGet() throws Exception {
    ExecutorService ex = Executors.newSingleThreadExecutor();
    try {
      return ex.submit(() -> new A()).get().run()
          + ex.submit(() -> new B()).get().run();
    } finally {
      ex.shutdown();
    }
  }

  public static int useVarSubmit() throws Exception {
    ExecutorService ex = Executors.newSingleThreadExecutor();
    try {
      var fa = ex.submit(() -> new A());
      var fb = ex.submit(() -> new B());
      return fa.get().run() + fb.get().run();
    } finally {
      ex.shutdown();
    }
  }

  public static int usePreservesB() throws Exception {
    ExecutorService ex = Executors.newSingleThreadExecutor();
    try {
      return ex.submit(() -> new B()).get().run();
    } finally {
      ex.shutdown();
    }
  }
}
