import java.util.HashMap;
import java.util.LinkedHashMap;
import java.util.Map;
import java.util.TreeMap;

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
  int useHashMap(BoxA ba, BoxB bb) {
    return new HashMap<>(Map.of("k", ba.get())).get("k").run()
        + new HashMap<>(Map.of("k", bb.get())).get("k").run();
  }

  int useLinkedHashMap(BoxA ba, BoxB bb) {
    return new LinkedHashMap<>(Map.of("k", ba.get())).get("k").run()
        + new LinkedHashMap<>(Map.of("k", bb.get())).get("k").run();
  }

  int useTreeMap(BoxA ba, BoxB bb) {
    return new TreeMap<>(Map.of("k", ba.get())).get("k").run()
        + new TreeMap<>(Map.of("k", bb.get())).get("k").run();
  }

  int useVar(BoxA ba, BoxB bb) {
    var ma = new HashMap<>(Map.of("k", ba.get()));
    var mb = new HashMap<>(Map.of("k", bb.get()));
    return ma.get("k").run() + mb.get("k").run();
  }

  // Class regression
  int useClass() {
    return new HashMap<>(Map.of("k", new A())).get("k").run()
        + new HashMap<>(Map.of("k", new B())).get("k").run();
  }

  int usePreservesB(BoxB bb) {
    return new HashMap<>(Map.of("k", bb.get())).get("k").run()
        + new HashMap<>(Map.of("k", new B())).get("k").run();
  }
}
