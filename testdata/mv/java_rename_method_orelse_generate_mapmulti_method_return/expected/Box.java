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
  int useOrElseGet(BoxA ba, BoxB bb) {
    return Optional.<A>empty().orElseGet(() -> ba.get()).execute()
        + Optional.<B>empty().orElseGet(() -> bb.get()).run();
  }

  int useOrElseGetAssign(BoxA ba, BoxB bb) {
    var xa = Optional.<A>empty().orElseGet(() -> ba.get());
    var xb = Optional.<B>empty().orElseGet(() -> bb.get());
    return xa.execute() + xb.run();
  }

  int useOrElse(BoxA ba, BoxB bb) {
    return Optional.<A>empty().orElse(ba.get()).execute()
        + Optional.<B>empty().orElse(bb.get()).run();
  }

  int useOrElseAssign(BoxA ba, BoxB bb) {
    var xa = Optional.<A>empty().orElse(ba.get());
    var xb = Optional.<B>empty().orElse(bb.get());
    return xa.execute() + xb.run();
  }

  int useGenerate(BoxA ba, BoxB bb) {
    return Stream.generate(() -> ba.get()).limit(1).findFirst().get().execute()
        + Stream.generate(() -> bb.get()).limit(1).findFirst().get().run();
  }

  int useGenerateAssign(BoxA ba, BoxB bb) {
    var xa = Stream.generate(() -> ba.get()).limit(1).findFirst().get();
    var xb = Stream.generate(() -> bb.get()).limit(1).findFirst().get();
    return xa.execute() + xb.run();
  }

  int useIterate(BoxA ba, BoxB bb) {
    return Stream.iterate(ba.get(), x -> x).limit(1).findFirst().get().execute()
        + Stream.iterate(bb.get(), x -> x).limit(1).findFirst().get().run();
  }

  int useIteratePred(BoxA ba, BoxB bb) {
    return Stream.iterate(ba.get(), x -> true, x -> x).limit(1).findFirst().get().execute()
        + Stream.iterate(bb.get(), x -> true, x -> x).limit(1).findFirst().get().run();
  }

  int useMapMultiId(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).mapMulti((a, c) -> c.accept(a)).findFirst().get().execute()
        + Stream.of(bb.get()).mapMulti((b, c) -> c.accept(b)).findFirst().get().run();
  }

  int useMapMultiAccept(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).mapMulti((a, c) -> c.accept(ba.get())).findFirst().get().execute()
        + Stream.of(bb.get()).mapMulti((b, c) -> c.accept(bb.get())).findFirst().get().run();
  }


  int useSupplyAsync(BoxA ba, BoxB bb) {
    return CompletableFuture.supplyAsync(() -> ba.get()).join().execute()
        + CompletableFuture.supplyAsync(() -> bb.get()).join().run();
  }

  int useWithInitial(BoxA ba, BoxB bb) {
    return ThreadLocal.withInitial(() -> ba.get()).get().execute()
        + ThreadLocal.withInitial(() -> bb.get()).get().run();
  }

  int useClass() {
    return Optional.<A>empty().orElseGet(() -> new A()).execute()
        + Optional.<B>empty().orElseGet(() -> new B()).run()
        + Optional.<A>empty().orElse(new A()).execute()
        + Optional.<B>empty().orElse(new B()).run()
        + Stream.generate(() -> new A()).limit(1).findFirst().get().execute()
        + Stream.generate(() -> new B()).limit(1).findFirst().get().run()
        + Stream.iterate(new A(), x -> x).limit(1).findFirst().get().execute()
        + Stream.iterate(new B(), x -> x).limit(1).findFirst().get().run()
        + CompletableFuture.supplyAsync(() -> new A()).join().execute()
        + CompletableFuture.supplyAsync(() -> new B()).join().run()
        + ThreadLocal.withInitial(() -> new A()).get().execute()
        + ThreadLocal.withInitial(() -> new B()).get().run();
  }

  int usePreservesB(BoxB bb) {
    return Optional.<B>empty().orElseGet(() -> bb.get()).run()
        + Optional.<B>empty().orElse(bb.get()).run()
        + Stream.generate(() -> bb.get()).limit(1).findFirst().get().run()
        + Stream.iterate(bb.get(), x -> x).limit(1).findFirst().get().run()
        + Stream.of(bb.get()).mapMulti((b, c) -> c.accept(b)).findFirst().get().run()
        + CompletableFuture.supplyAsync(() -> bb.get()).join().run()
        + ThreadLocal.withInitial(() -> bb.get()).get().run();
  }
}
