import java.util.concurrent.atomic.AtomicReference;
import java.lang.ref.WeakReference;
import java.lang.ref.SoftReference;

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
  int useAtomic(BoxA ba, BoxB bb) {
    return new AtomicReference<>(ba.get()).get().execute()
        + new AtomicReference<>(bb.get()).get().run();
  }

  int useWeak(BoxA ba, BoxB bb) {
    return new WeakReference<>(ba.get()).get().execute()
        + new WeakReference<>(bb.get()).get().run();
  }

  int useSoft(BoxA ba, BoxB bb) {
    return new SoftReference<>(ba.get()).get().execute()
        + new SoftReference<>(bb.get()).get().run();
  }

  int useAssign(BoxA ba, BoxB bb) {
    var ar = new AtomicReference<>(ba.get());
    var br = new AtomicReference<>(bb.get());
    return ar.get().execute() + br.get().run();
  }

  int useClass() {
    return new AtomicReference<>(new A()).get().execute()
        + new AtomicReference<>(new B()).get().run();
  }

  int usePreservesB(BoxB bb) {
    return new AtomicReference<>(bb.get()).get().run()
        + new WeakReference<>(bb.get()).get().run();
  }
}
