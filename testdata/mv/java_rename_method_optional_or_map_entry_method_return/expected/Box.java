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
  int useOptionalOfGet(BoxA ba, BoxB bb) {
    return Optional.of(ba.get()).get().execute() + Optional.of(bb.get()).get().run();
  }

  int useOptionalOr(BoxA ba, BoxB bb) {
    return Optional.of(ba.get()).or(() -> Optional.empty()).orElseThrow().execute()
        + Optional.of(bb.get()).or(() -> Optional.empty()).orElseThrow().run();
  }

  int useMapEntry(BoxA ba, BoxB bb) {
    return Map.entry("k", ba.get()).getValue().execute()
        + Map.entry("k", bb.get()).getValue().run();
  }

  int useMapEntrySetValue(BoxA ba, BoxB bb) {
    return Map.entry("k", ba.get()).setValue(ba.get()).execute()
        + Map.entry("k", bb.get()).setValue(bb.get()).run();
  }

  int usePreservesB(BoxB bb) {
    return Optional.of(bb.get()).get().run()
        + Optional.of(bb.get()).or(() -> Optional.empty()).orElseThrow().run()
        + Map.entry("k", bb.get()).getValue().run();
  }
}
