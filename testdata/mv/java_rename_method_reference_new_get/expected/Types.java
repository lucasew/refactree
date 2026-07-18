package demo;

import java.lang.ref.WeakReference;
import java.util.concurrent.atomic.AtomicReference;

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
  public static int useNewAtomicGet() {
    return new AtomicReference<>(new A()).get().execute()
        + new AtomicReference<>(new B()).get().run();
  }

  public static int useNewWeakGet() {
    return new WeakReference<>(new A()).get().execute()
        + new WeakReference<>(new B()).get().run();
  }

  public static int useVarAtomicGet() {
    var aa = new AtomicReference<>(new A());
    var ab = new AtomicReference<>(new B());
    return aa.get().execute() + ab.get().run();
  }

  public static int useVarWeakGet() {
    var wa = new WeakReference<>(new A());
    var wb = new WeakReference<>(new B());
    return wa.get().execute() + wb.get().run();
  }

  public static int usePreservesB() {
    return new AtomicReference<>(new B()).get().run()
        + new WeakReference<>(new B()).get().run();
  }
}
