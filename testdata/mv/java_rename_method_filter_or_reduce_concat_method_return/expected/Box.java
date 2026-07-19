import java.util.Optional;
import java.util.stream.Stream;
import java.util.concurrent.CompletableFuture;

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
  int useFilter(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).filter(x -> true).findFirst().get().execute()
        + Stream.of(bb.get()).filter(x -> true).findFirst().get().run();
  }

  int useFilterAssign(BoxA ba, BoxB bb) {
    var xa = Stream.of(ba.get()).filter(x -> true).findFirst().get();
    var xb = Stream.of(bb.get()).filter(x -> true).findFirst().get();
    return xa.execute() + xb.run();
  }

  int useOr(BoxA ba, BoxB bb) {
    return Optional.of(ba.get()).or(() -> Optional.empty()).get().execute()
        + Optional.of(bb.get()).or(() -> Optional.empty()).get().run();
  }

  int useOrAssign(BoxA ba, BoxB bb) {
    var xa = Optional.of(ba.get()).or(() -> Optional.empty()).get();
    var xb = Optional.of(bb.get()).or(() -> Optional.empty()).get();
    return xa.execute() + xb.run();
  }

  int useReduceOpt(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).reduce((a, b) -> a).get().execute()
        + Stream.of(bb.get()).reduce((a, b) -> a).get().run();
  }

  int useReduceId(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).reduce(ba.get(), (a, b) -> a).execute()
        + Stream.of(bb.get()).reduce(bb.get(), (a, b) -> a).run();
  }

  int useMin(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).min((a, b) -> 0).get().execute()
        + Stream.of(bb.get()).min((a, b) -> 0).get().run();
  }

  int useConcat(BoxA ba, BoxB bb) {
    return Stream.concat(Stream.of(ba.get()), Stream.of(ba.get())).findFirst().get().execute()
        + Stream.concat(Stream.of(bb.get()), Stream.of(bb.get())).findFirst().get().run();
  }

  int useThenApply(BoxA ba, BoxB bb) {
    return CompletableFuture.completedFuture(ba.get()).thenApply(x -> x).join().execute()
        + CompletableFuture.completedFuture(bb.get()).thenApply(x -> x).join().run();
  }

  int useClass() {
    return Stream.of(new A()).filter(x -> true).findFirst().get().execute()
        + Stream.of(new B()).filter(x -> true).findFirst().get().run()
        + Optional.of(new A()).or(() -> Optional.empty()).get().execute()
        + Optional.of(new B()).or(() -> Optional.empty()).get().run()
        + Stream.concat(Stream.of(new A()), Stream.of(new A())).findFirst().get().execute()
        + Stream.concat(Stream.of(new B()), Stream.of(new B())).findFirst().get().run();
  }

  int usePreservesB(BoxB bb) {
    return Stream.of(bb.get()).filter(x -> true).findFirst().get().run()
        + Optional.of(bb.get()).or(() -> Optional.empty()).get().run()
        + Stream.of(bb.get()).reduce((a, b) -> a).get().run()
        + Stream.concat(Stream.of(bb.get()), Stream.of(bb.get())).findFirst().get().run()
        + CompletableFuture.completedFuture(bb.get()).thenApply(x -> x).join().run();
  }

  int useDirect(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).findFirst().get().execute()
        + Optional.of(ba.get()).get().execute()
        + Stream.of(bb.get()).findFirst().get().run()
        + Optional.of(bb.get()).get().run();
  }
}
