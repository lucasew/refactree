import java.util.HashMap;
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
  // Map.of(method-return).getOrDefault under foreign same-leaf.
  int useGetOrDefault(BoxA ba, BoxB bb) {
    return Map.of("k", ba.get()).getOrDefault("k", null).run()
        + Map.of("k", bb.get()).getOrDefault("k", null).run();
  }

  int useGetOrDefaultVar(BoxA ba, BoxB bb) {
    var xa = Map.of("k", ba.get()).getOrDefault("k", null);
    var xb = Map.of("k", bb.get()).getOrDefault("k", null);
    return xa.run() + xb.run();
  }

  // NavigableMap entry accessors on TreeMap copy of Map.of(method-return).
  int useFirstEntry(BoxA ba, BoxB bb) {
    return new TreeMap<>(Map.of("k", ba.get())).firstEntry().getValue().run()
        + new TreeMap<>(Map.of("k", bb.get())).firstEntry().getValue().run();
  }

  int useLastEntry(BoxA ba, BoxB bb) {
    return new TreeMap<>(Map.of("k", ba.get())).lastEntry().getValue().run()
        + new TreeMap<>(Map.of("k", bb.get())).lastEntry().getValue().run();
  }

  int usePollFirstEntry(BoxA ba, BoxB bb) {
    return new TreeMap<>(Map.of("k", ba.get())).pollFirstEntry().getValue().run()
        + new TreeMap<>(Map.of("k", bb.get())).pollFirstEntry().getValue().run();
  }

  int useCeilingEntry(BoxA ba, BoxB bb) {
    return new TreeMap<>(Map.of("k", ba.get())).ceilingEntry("k").getValue().run()
        + new TreeMap<>(Map.of("k", bb.get())).ceilingEntry("k").getValue().run();
  }

  int useFloorEntry(BoxA ba, BoxB bb) {
    return new TreeMap<>(Map.of("k", ba.get())).floorEntry("k").getValue().run()
        + new TreeMap<>(Map.of("k", bb.get())).floorEntry("k").getValue().run();
  }

  // values() / entrySet() pipelines of Map.of(method-return).
  int useValuesNext(BoxA ba, BoxB bb) {
    return Map.of("k", ba.get()).values().iterator().next().run()
        + Map.of("k", bb.get()).values().iterator().next().run();
  }

  int useValuesStream(BoxA ba, BoxB bb) {
    return Map.of("k", ba.get()).values().stream().findFirst().get().run()
        + Map.of("k", bb.get()).values().stream().findFirst().get().run();
  }

  int useEntrySetNext(BoxA ba, BoxB bb) {
    return Map.of("k", ba.get()).entrySet().iterator().next().getValue().run()
        + Map.of("k", bb.get()).entrySet().iterator().next().getValue().run();
  }

  int useEntrySetStream(BoxA ba, BoxB bb) {
    return Map.of("k", ba.get()).entrySet().stream().findFirst().get().getValue().run()
        + Map.of("k", bb.get()).entrySet().stream().findFirst().get().getValue().run();
  }

  // put return on map copy of method-return values.
  int usePutReturn(BoxA ba, BoxB bb) {
    return new HashMap<>(Map.of("k", ba.get())).put("k", ba.get()).run()
        + new HashMap<>(Map.of("k", bb.get())).put("k", bb.get()).run();
  }

  // Class regression — already worked.
  int useClass() {
    return Map.of("k", new A()).getOrDefault("k", null).run()
        + Map.of("k", new B()).getOrDefault("k", null).run()
        + new TreeMap<>(Map.of("k", new A())).firstEntry().getValue().run()
        + new TreeMap<>(Map.of("k", new B())).firstEntry().getValue().run()
        + Map.of("k", new A()).values().iterator().next().run()
        + Map.of("k", new B()).values().iterator().next().run()
        + Map.of("k", new A()).entrySet().iterator().next().getValue().run()
        + Map.of("k", new B()).entrySet().iterator().next().getValue().run();
  }

  int usePreservesB(BoxB bb) {
    return Map.of("k", bb.get()).getOrDefault("k", null).run()
        + new TreeMap<>(Map.of("k", bb.get())).firstEntry().getValue().run()
        + Map.of("k", bb.get()).values().iterator().next().run()
        + Map.of("k", bb.get()).entrySet().iterator().next().getValue().run()
        + Map.of("k", new B()).getOrDefault("k", null).run();
  }
}
