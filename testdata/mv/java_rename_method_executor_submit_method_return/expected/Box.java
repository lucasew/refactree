import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

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
  A held = new A();

  A get() {
    return held;
  }
}

class BoxB {
  B held = new B();

  B get() {
    return held;
  }
}

class Use {
  // ExecutorService.submit(Callable) method-return under foreign same-leaf.
  int useSubmit(BoxA ba, BoxB bb) throws Exception {
    ExecutorService es = Executors.newSingleThreadExecutor();
    try {
      return es.submit(() -> ba.get()).get().execute()
          + es.submit(() -> bb.get()).get().run();
    } finally {
      es.shutdown();
    }
  }

  int useVar(BoxA ba, BoxB bb) throws Exception {
    ExecutorService es = Executors.newSingleThreadExecutor();
    try {
      var fa = es.submit(() -> ba.get());
      var fb = es.submit(() -> bb.get());
      return fa.get().execute() + fb.get().run();
    } finally {
      es.shutdown();
    }
  }

  // Class regression — already worked.
  int useClass() throws Exception {
    ExecutorService es = Executors.newSingleThreadExecutor();
    try {
      return es.submit(() -> new A()).get().execute()
          + es.submit(() -> new B()).get().run();
    } finally {
      es.shutdown();
    }
  }

  int usePreservesB(BoxB bb) throws Exception {
    ExecutorService es = Executors.newSingleThreadExecutor();
    try {
      return es.submit(() -> bb.get()).get().run()
          + es.submit(() -> new B()).get().run();
    } finally {
      es.shutdown();
    }
  }
}
