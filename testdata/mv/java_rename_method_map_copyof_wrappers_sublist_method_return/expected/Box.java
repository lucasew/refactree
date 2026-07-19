import java.util.Collections;
import java.util.List;
import java.util.Map;
import java.util.TreeMap;

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
  // Map.copyOf(Map.of(k, method-return)) under foreign same-leaf.
  int useMapCopyOf(BoxA ba, BoxB bb) {
    return Map.copyOf(Map.of("k", ba.get())).get("k").execute()
        + Map.copyOf(Map.of("k", bb.get())).get("k").run();
  }

  int useMapCopyOfVar(BoxA ba, BoxB bb) {
    var ma = Map.copyOf(Map.of("k", ba.get()));
    var mb = Map.copyOf(Map.of("k", bb.get()));
    return ma.get("k").execute() + mb.get("k").run();
  }

  // Collections map wrappers over Map.of method-return.
  int useUnmodifiableMap(BoxA ba, BoxB bb) {
    return Collections.unmodifiableMap(Map.of("k", ba.get())).get("k").execute()
        + Collections.unmodifiableMap(Map.of("k", bb.get())).get("k").run();
  }

  int useSynchronizedMap(BoxA ba, BoxB bb) {
    return Collections.synchronizedMap(Map.of("k", ba.get())).get("k").execute()
        + Collections.synchronizedMap(Map.of("k", bb.get())).get("k").run();
  }

  int useCheckedMap(BoxA ba, BoxB bb) {
    return Collections.checkedMap(Map.of("k", ba.get()), String.class, A.class).get("k").execute()
        + Collections.checkedMap(Map.of("k", bb.get()), String.class, B.class).get("k").run();
  }

  int useUnmodifiableSortedMap(BoxA ba, BoxB bb) {
    return Collections.unmodifiableSortedMap(new TreeMap<>(Map.of("k", ba.get()))).get("k").execute()
        + Collections.unmodifiableSortedMap(new TreeMap<>(Map.of("k", bb.get()))).get("k").run();
  }

  // List.of(method-return).subList — Class already solid; MR was UNDER inline.
  int useSubList(BoxA ba, BoxB bb) {
    return List.of(ba.get()).subList(0, 1).get(0).execute()
        + List.of(bb.get()).subList(0, 1).get(0).run();
  }

  int useSubListVar(BoxA ba, BoxB bb) {
    var xa = List.of(ba.get()).subList(0, 1);
    var xb = List.of(bb.get()).subList(0, 1);
    return xa.get(0).execute() + xb.get(0).run();
  }

  // Class regression — already worked.
  int useClass() {
    return Map.copyOf(Map.of("k", new A())).get("k").execute()
        + Map.copyOf(Map.of("k", new B())).get("k").run()
        + Collections.unmodifiableMap(Map.of("k", new A())).get("k").execute()
        + Collections.unmodifiableMap(Map.of("k", new B())).get("k").run()
        + List.of(new A()).subList(0, 1).get(0).execute()
        + List.of(new B()).subList(0, 1).get(0).run();
  }

  int usePreservesB(BoxB bb) {
    return Map.copyOf(Map.of("k", bb.get())).get("k").run()
        + Collections.unmodifiableMap(Map.of("k", bb.get())).get("k").run()
        + List.of(bb.get()).subList(0, 1).get(0).run()
        + Map.copyOf(Map.of("k", new B())).get("k").run();
  }
}
