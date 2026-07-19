import java.util.stream.Gatherers;
import java.util.stream.Stream;

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
  // Stream.of(method-return).gather(mapConcurrent identity) under foreign same-leaf.
  int useMapConcurrent(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).gather(Gatherers.mapConcurrent(1, a -> a)).findFirst().get().execute()
        + Stream.of(bb.get()).gather(Gatherers.mapConcurrent(1, b -> b)).findFirst().get().run();
  }

  int useVar(BoxA ba, BoxB bb) {
    var sa = Stream.of(ba.get()).gather(Gatherers.mapConcurrent(1, a -> a));
    var sb = Stream.of(bb.get()).gather(Gatherers.mapConcurrent(1, b -> b));
    return sa.findFirst().get().execute() + sb.findFirst().get().run();
  }

  // Class regression — already worked.
  int useClass() {
    return Stream.of(new A()).gather(Gatherers.mapConcurrent(1, a -> a)).findFirst().get().execute()
        + Stream.of(new B()).gather(Gatherers.mapConcurrent(1, b -> b)).findFirst().get().run();
  }

  int usePreservesB(BoxB bb) {
    return Stream.of(bb.get()).gather(Gatherers.mapConcurrent(1, b -> b)).findFirst().get().run()
        + Stream.of(new B()).gather(Gatherers.mapConcurrent(1, b -> b)).findFirst().get().run();
  }
}
