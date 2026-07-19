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
  int useListOf(BoxA ba, BoxB bb) {
    return List.of(ba.get()).get(0).run() + List.of(bb.get()).get(0).run();
  }

  int useArraysAsList(BoxA ba, BoxB bb) {
    return Arrays.asList(ba.get()).get(0).run() + Arrays.asList(bb.get()).get(0).run();
  }

  int useSingletonList(BoxA ba, BoxB bb) {
    return Collections.singletonList(ba.get()).get(0).run()
        + Collections.singletonList(bb.get()).get(0).run();
  }

  int useNCopies(BoxA ba, BoxB bb) {
    return Collections.nCopies(1, ba.get()).get(0).run()
        + Collections.nCopies(1, bb.get()).get(0).run();
  }

  int useStreamOfFindFirst(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).findFirst().get().run()
        + Stream.of(bb.get()).findFirst().get().run();
  }

  int useStreamOfToList(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).toList().get(0).run()
        + Stream.of(bb.get()).toList().get(0).run();
  }

  int useMapOf(BoxA ba, BoxB bb) {
    return Map.of("k", ba.get()).get("k").run() + Map.of("k", bb.get()).get("k").run();
  }

  int useCompletedFuture(BoxA ba, BoxB bb) {
    return CompletableFuture.completedFuture(ba.get()).join().run()
        + CompletableFuture.completedFuture(bb.get()).join().run();
  }

  int useIdentity(BoxA ba, BoxB bb) {
    return Function.identity().apply(ba.get()).run()
        + Function.identity().apply(bb.get()).run();
  }

  int useIteratorNext(BoxA ba, BoxB bb) {
    return List.of(ba.get()).iterator().next().run()
        + List.of(bb.get()).iterator().next().run();
  }

  int usePreservesB(BoxB bb) {
    return List.of(bb.get()).get(0).run()
        + Stream.of(bb.get()).findFirst().get().run()
        + Map.of("k", bb.get()).get("k").run()
        + Function.identity().apply(bb.get()).run()
        + Collections.nCopies(1, bb.get()).get(0).run();
  }
}
