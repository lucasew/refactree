import java.util.Map;
import java.util.stream.Collectors;
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
  // Stream.of(method-return).collect(groupingBy).get(k).get(0) under foreign same-leaf.
  int useGroupingByGet(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).collect(Collectors.groupingBy(a -> "k")).get("k").get(0).execute()
        + Stream.of(bb.get()).collect(Collectors.groupingBy(b -> "k")).get("k").get(0).run();
  }

  int useGroupingByGetVar(BoxA ba, BoxB bb) {
    var ma = Stream.of(ba.get()).collect(Collectors.groupingBy(a -> "k"));
    var mb = Stream.of(bb.get()).collect(Collectors.groupingBy(b -> "k"));
    return ma.get("k").get(0).execute() + mb.get("k").get(0).run();
  }

  int useGroupingByListVar(BoxA ba, BoxB bb) {
    var ga = Stream.of(ba.get()).collect(Collectors.groupingBy(a -> "k")).get("k");
    var gb = Stream.of(bb.get()).collect(Collectors.groupingBy(b -> "k")).get("k");
    return ga.get(0).execute() + gb.get(0).run();
  }

  int useGroupingByDownstream(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).collect(Collectors.groupingBy(a -> "k", Collectors.toList())).get("k").get(0).execute()
        + Stream.of(bb.get()).collect(Collectors.groupingBy(b -> "k", Collectors.toList())).get("k").get(0).run();
  }

  int useGroupingByEntry(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).collect(Collectors.groupingBy(a -> "k")).entrySet().iterator().next().getValue().get(0).execute()
        + Stream.of(bb.get()).collect(Collectors.groupingBy(b -> "k")).entrySet().iterator().next().getValue().get(0).run();
  }

  int useGroupingByValuesForEach(BoxA ba, BoxB bb) {
    int[] n = {0};
    Stream.of(ba.get()).collect(Collectors.groupingBy(a -> "k")).values().forEach(g -> g.forEach(xa -> n[0] += xa.execute()));
    Stream.of(bb.get()).collect(Collectors.groupingBy(b -> "k")).values().forEach(g -> g.forEach(xb -> n[0] += xb.run()));
    return n[0];
  }

  int usePartitioningByGet(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).collect(Collectors.partitioningBy(a -> true)).get(true).get(0).execute()
        + Stream.of(bb.get()).collect(Collectors.partitioningBy(b -> true)).get(true).get(0).run();
  }

  int usePartitioningByDownstream(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).collect(Collectors.partitioningBy(a -> true, Collectors.toList())).get(true).get(0).execute()
        + Stream.of(bb.get()).collect(Collectors.partitioningBy(b -> true, Collectors.toList())).get(true).get(0).run();
  }

  int usePartitioningByVar(BoxA ba, BoxB bb) {
    var ma = Stream.of(ba.get()).collect(Collectors.partitioningBy(a -> true));
    var mb = Stream.of(bb.get()).collect(Collectors.partitioningBy(b -> true));
    return ma.get(true).get(0).execute() + mb.get(true).get(0).run();
  }

  // Class regression — already worked.
  int useClass() {
    return Stream.of(new A()).collect(Collectors.groupingBy(a -> "k")).get("k").get(0).execute()
        + Stream.of(new B()).collect(Collectors.groupingBy(b -> "k")).get("k").get(0).run()
        + Stream.of(new A()).collect(Collectors.partitioningBy(a -> true)).get(true).get(0).execute()
        + Stream.of(new B()).collect(Collectors.partitioningBy(b -> true)).get(true).get(0).run()
        + Stream.of(new A()).collect(Collectors.groupingBy(a -> "k")).entrySet().iterator().next().getValue().get(0).execute()
        + Stream.of(new B()).collect(Collectors.groupingBy(b -> "k")).entrySet().iterator().next().getValue().get(0).run();
  }

  int usePreservesB(BoxB bb) {
    return Stream.of(bb.get()).collect(Collectors.groupingBy(b -> "k")).get("k").get(0).run()
        + Stream.of(new B()).collect(Collectors.groupingBy(b -> "k")).get("k").get(0).run()
        + Stream.of(bb.get()).collect(Collectors.partitioningBy(b -> true)).get(true).get(0).run()
        + Stream.of(new B()).collect(Collectors.partitioningBy(b -> true)).get(true).get(0).run();
  }
}
