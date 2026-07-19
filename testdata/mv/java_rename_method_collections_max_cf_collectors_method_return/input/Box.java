import java.util.Collections;
import java.util.Comparator;
import java.util.List;
import java.util.concurrent.CompletableFuture;
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
  int useCollectionsMax(BoxA ba, BoxB bb) {
    return Collections.max(List.of(ba.get()), Comparator.comparingInt(x -> 0)).run()
        + Collections.max(List.of(bb.get()), Comparator.comparingInt(x -> 0)).run();
  }

  int useCollectionsMin(BoxA ba, BoxB bb) {
    return Collections.min(List.of(ba.get()), Comparator.comparingInt(x -> 0)).run()
        + Collections.min(List.of(bb.get()), Comparator.comparingInt(x -> 0)).run();
  }

  int useCollectionsMaxClass() {
    return Collections.max(List.of(new A()), Comparator.comparingInt(x -> 0)).run()
        + Collections.max(List.of(new B()), Comparator.comparingInt(x -> 0)).run();
  }

  int useCollectionsMaxAssign(BoxA ba, BoxB bb) {
    var xa = Collections.max(List.of(ba.get()), Comparator.comparingInt(x -> 0));
    var xb = Collections.max(List.of(bb.get()), Comparator.comparingInt(x -> 0));
    return xa.run() + xb.run();
  }

  int useThenCompose(BoxA ba, BoxB bb) {
    return CompletableFuture.completedFuture(ba.get())
            .thenCompose(x -> CompletableFuture.completedFuture(x))
            .join()
            .run()
        + CompletableFuture.completedFuture(bb.get())
            .thenCompose(x -> CompletableFuture.completedFuture(x))
            .join()
            .run();
  }

  int useHandle(BoxA ba, BoxB bb) {
    return CompletableFuture.completedFuture(ba.get()).handle((x, e) -> x).join().run()
        + CompletableFuture.completedFuture(bb.get()).handle((x, e) -> x).join().run();
  }

  int useWhenComplete(BoxA ba, BoxB bb) {
    return CompletableFuture.completedFuture(ba.get()).whenComplete((x, e) -> {}).join().run()
        + CompletableFuture.completedFuture(bb.get()).whenComplete((x, e) -> {}).join().run();
  }

  int useExceptionally(BoxA ba, BoxB bb) {
    return CompletableFuture.completedFuture(ba.get()).exceptionally(e -> ba.get()).join().run()
        + CompletableFuture.completedFuture(bb.get()).exceptionally(e -> bb.get()).join().run();
  }

  int useHandleClass() {
    return CompletableFuture.completedFuture(new A()).handle((x, e) -> x).join().run()
        + CompletableFuture.completedFuture(new B()).handle((x, e) -> x).join().run();
  }

  int useHandleAssign(BoxA ba, BoxB bb) {
    var xa = CompletableFuture.completedFuture(ba.get()).handle((x, e) -> x).join();
    var xb = CompletableFuture.completedFuture(bb.get()).handle((x, e) -> x).join();
    return xa.run() + xb.run();
  }

  int useReducing(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).collect(Collectors.reducing((a, b) -> a)).get().run()
        + Stream.of(bb.get()).collect(Collectors.reducing((a, b) -> a)).get().run();
  }

  int useMaxBy(BoxA ba, BoxB bb) {
    return Stream.of(ba.get())
            .collect(Collectors.maxBy(Comparator.comparingInt(x -> 0)))
            .get()
            .run()
        + Stream.of(bb.get())
            .collect(Collectors.maxBy(Comparator.comparingInt(x -> 0)))
            .get()
            .run();
  }

  int useMinBy(BoxA ba, BoxB bb) {
    return Stream.of(ba.get())
            .collect(Collectors.minBy(Comparator.comparingInt(x -> 0)))
            .get()
            .run()
        + Stream.of(bb.get())
            .collect(Collectors.minBy(Comparator.comparingInt(x -> 0)))
            .get()
            .run();
  }

  int useCollectingAndThen(BoxA ba, BoxB bb) {
    return Stream.of(ba.get())
            .collect(Collectors.collectingAndThen(Collectors.toList(), l -> l.get(0)))
            .run()
        + Stream.of(bb.get())
            .collect(Collectors.collectingAndThen(Collectors.toList(), l -> l.get(0)))
            .run();
  }

  int useMaxByClass() {
    return Stream.of(new A())
            .collect(Collectors.maxBy(Comparator.comparingInt(x -> 0)))
            .get()
            .run()
        + Stream.of(new B())
            .collect(Collectors.maxBy(Comparator.comparingInt(x -> 0)))
            .get()
            .run();
  }

  int useReducingAssign(BoxA ba, BoxB bb) {
    var xa = Stream.of(ba.get()).collect(Collectors.reducing((a, b) -> a)).get();
    var xb = Stream.of(bb.get()).collect(Collectors.reducing((a, b) -> a)).get();
    return xa.run() + xb.run();
  }
}
