import java.util.AbstractMap;
import java.util.Collections;
import java.util.Map;

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
  // Map.ofEntries(Map.entry(k, method-return)) under foreign same-leaf.
  int useOfEntries(BoxA ba, BoxB bb) {
    return Map.ofEntries(Map.entry("k", ba.get())).get("k").execute()
        + Map.ofEntries(Map.entry("k", bb.get())).get("k").run();
  }

  // AbstractMap.SimpleEntry / SimpleImmutableEntry diamond method-return.
  int useSimpleEntry(BoxA ba, BoxB bb) {
    return new AbstractMap.SimpleEntry<>("k", ba.get()).getValue().execute()
        + new AbstractMap.SimpleEntry<>("k", bb.get()).getValue().run();
  }

  int useSimpleImm(BoxA ba, BoxB bb) {
    return new AbstractMap.SimpleImmutableEntry<>("k", ba.get()).getValue().execute()
        + new AbstractMap.SimpleImmutableEntry<>("k", bb.get()).getValue().run();
  }

  int useVarSimpleEntry(BoxA ba, BoxB bb) {
    var ea = new AbstractMap.SimpleEntry<>("k", ba.get());
    var eb = new AbstractMap.SimpleEntry<>("k", bb.get());
    return ea.getValue().execute() + eb.getValue().run();
  }

  // Collections.singletonMap(k, method-return).
  int useSingletonMap(BoxA ba, BoxB bb) {
    return Collections.singletonMap("k", ba.get()).get("k").execute()
        + Collections.singletonMap("k", bb.get()).get("k").run();
  }

  int useVarSingleton(BoxA ba, BoxB bb) {
    var ma = Collections.singletonMap("k", ba.get());
    var mb = Collections.singletonMap("k", bb.get());
    return ma.get("k").execute() + mb.get("k").run();
  }

  // Class regression — already worked.
  int useClass() {
    return Map.ofEntries(Map.entry("k", new A())).get("k").execute()
        + Map.ofEntries(Map.entry("k", new B())).get("k").run()
        + new AbstractMap.SimpleEntry<>("k", new A()).getValue().execute()
        + new AbstractMap.SimpleEntry<>("k", new B()).getValue().run()
        + Collections.singletonMap("k", new A()).get("k").execute()
        + Collections.singletonMap("k", new B()).get("k").run();
  }

  int usePreservesB(BoxB bb) {
    return Map.ofEntries(Map.entry("k", bb.get())).get("k").run()
        + new AbstractMap.SimpleEntry<>("k", bb.get()).getValue().run()
        + Collections.singletonMap("k", bb.get()).get("k").run()
        + Map.ofEntries(Map.entry("k", new B())).get("k").run();
  }
}
