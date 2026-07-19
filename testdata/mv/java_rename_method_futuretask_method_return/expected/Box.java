import java.util.concurrent.FutureTask;

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
  // FutureTask diamond Callable / Runnable+result method-return under foreign same-leaf.
  int useCallable(BoxA ba, BoxB bb) throws Exception {
    return new FutureTask<>(() -> ba.get()).get().execute()
        + new FutureTask<>(() -> bb.get()).get().run();
  }

  int useRunnableResult(BoxA ba, BoxB bb) throws Exception {
    return new FutureTask<>(() -> {}, ba.get()).get().execute()
        + new FutureTask<>(() -> {}, bb.get()).get().run();
  }

  int useVar(BoxA ba, BoxB bb) throws Exception {
    var fa = new FutureTask<>(() -> ba.get());
    var fb = new FutureTask<>(() -> bb.get());
    return fa.get().execute() + fb.get().run();
  }

  // Class regression — already worked.
  int useClass() throws Exception {
    return new FutureTask<>(() -> new A()).get().execute()
        + new FutureTask<>(() -> new B()).get().run()
        + new FutureTask<>(() -> {}, new A()).get().execute()
        + new FutureTask<>(() -> {}, new B()).get().run();
  }

  int usePreservesB(BoxB bb) throws Exception {
    return new FutureTask<>(() -> bb.get()).get().run()
        + new FutureTask<>(() -> {}, bb.get()).get().run()
        + new FutureTask<>(() -> new B()).get().run();
  }
}
