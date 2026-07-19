import java.util.Map;
import java.util.stream.Collectors;
import java.util.stream.Stream;

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
  // Stream.of(method-return).collect(toMap identity).get under foreign same-leaf.
  int useGet(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).collect(Collectors.toMap(a -> "k", a -> a)).get("k").run()
        + Stream.of(bb.get()).collect(Collectors.toMap(b -> "k", b -> b)).get("k").run();
  }

  int useGetVar(BoxA ba, BoxB bb) {
    var ma = Stream.of(ba.get()).collect(Collectors.toMap(a -> "k", a -> a));
    var mb = Stream.of(bb.get()).collect(Collectors.toMap(b -> "k", b -> b));
    return ma.get("k").run() + mb.get("k").run();
  }

  int useValues(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).collect(Collectors.toMap(a -> "k", a -> a)).values().iterator().next().run()
        + Stream.of(bb.get()).collect(Collectors.toMap(b -> "k", b -> b)).values().iterator().next().run();
  }

  int useValuesForEach(BoxA ba, BoxB bb) {
    int[] n = {0};
    Stream.of(ba.get()).collect(Collectors.toMap(a -> "k", a -> a)).values().forEach(xa -> n[0] += xa.run());
    Stream.of(bb.get()).collect(Collectors.toMap(b -> "k", b -> b)).values().forEach(xb -> n[0] += xb.run());
    return n[0];
  }

  int useToConcurrentMap(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).collect(Collectors.toConcurrentMap(a -> "k", a -> a)).get("k").run()
        + Stream.of(bb.get()).collect(Collectors.toConcurrentMap(b -> "k", b -> b)).get("k").run();
  }

  int useToUnmodifiableMap(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).collect(Collectors.toUnmodifiableMap(a -> "k", a -> a)).get("k").run()
        + Stream.of(bb.get()).collect(Collectors.toUnmodifiableMap(b -> "k", b -> b)).get("k").run();
  }

  // Class regression — already worked.
  int useClass() {
    return Stream.of(new A()).collect(Collectors.toMap(a -> "k", a -> a)).get("k").run()
        + Stream.of(new B()).collect(Collectors.toMap(b -> "k", b -> b)).get("k").run()
        + Stream.of(new A()).collect(Collectors.toMap(a -> "k", a -> a)).values().iterator().next().run()
        + Stream.of(new B()).collect(Collectors.toMap(b -> "k", b -> b)).values().iterator().next().run();
  }

  int usePreservesB(BoxB bb) {
    return Stream.of(bb.get()).collect(Collectors.toMap(b -> "k", b -> b)).get("k").run()
        + Stream.of(new B()).collect(Collectors.toMap(b -> "k", b -> b)).get("k").run();
  }
}
