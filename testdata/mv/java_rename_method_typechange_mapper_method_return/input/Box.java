import java.util.Optional;
import java.util.concurrent.CompletableFuture;
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
  // Type-changing Stream/Optional.map method-return under foreign same-leaf.
  int useStreamMap(BoxA ba, BoxB bb) {
    return Stream.of(0).map(x -> ba.get()).findFirst().get().run()
        + Stream.of(0).map(x -> bb.get()).findFirst().get().run();
  }

  int useOptionalMap(BoxA ba, BoxB bb) {
    return Optional.of(0).map(x -> ba.get()).get().run()
        + Optional.of(0).map(x -> bb.get()).get().run();
  }

  // Type-changing CF thenApply / thenApplyAsync / handle method-return.
  int useThenApply(BoxA ba, BoxB bb) throws Exception {
    return CompletableFuture.completedFuture(0).thenApply(x -> ba.get()).join().run()
        + CompletableFuture.completedFuture(0).thenApply(x -> bb.get()).join().run();
  }

  int useThenApplyAsync(BoxA ba, BoxB bb) throws Exception {
    return CompletableFuture.completedFuture(0).thenApplyAsync(x -> ba.get()).join().run()
        + CompletableFuture.completedFuture(0).thenApplyAsync(x -> bb.get()).join().run();
  }

  int useHandle(BoxA ba, BoxB bb) throws Exception {
    return CompletableFuture.completedFuture(0).handle((x, e) -> ba.get()).join().run()
        + CompletableFuture.completedFuture(0).handle((x, e) -> bb.get()).join().run();
  }

  // Type-changing flatMap / thenCompose rewrap method-return.
  int useStreamFlatMap(BoxA ba, BoxB bb) {
    return Stream.of(0).flatMap(x -> Stream.of(ba.get())).findFirst().get().run()
        + Stream.of(0).flatMap(x -> Stream.of(bb.get())).findFirst().get().run();
  }

  int useOptionalFlatMap(BoxA ba, BoxB bb) {
    return Optional.of(0).flatMap(x -> Optional.of(ba.get())).get().run()
        + Optional.of(0).flatMap(x -> Optional.of(bb.get())).get().run();
  }

  int useThenCompose(BoxA ba, BoxB bb) throws Exception {
    return CompletableFuture.completedFuture(0)
            .thenCompose(x -> CompletableFuture.completedFuture(ba.get()))
            .join()
            .run()
        + CompletableFuture.completedFuture(0)
            .thenCompose(x -> CompletableFuture.completedFuture(bb.get()))
            .join()
            .run();
  }

  // applyToEither / thenCombine type-changing method-return.
  int useApplyToEither(BoxA ba, BoxB bb, CompletableFuture<Integer> o) throws Exception {
    return CompletableFuture.completedFuture(0).applyToEither(o, x -> ba.get()).join().run()
        + CompletableFuture.completedFuture(0).applyToEither(o, x -> bb.get()).join().run();
  }

  int useThenCombine(BoxA ba, BoxB bb, CompletableFuture<Integer> o) throws Exception {
    return CompletableFuture.completedFuture(0).thenCombine(o, (a, b) -> ba.get()).join().run()
        + CompletableFuture.completedFuture(0).thenCombine(o, (a, b) -> bb.get()).join().run();
  }

  // Identity applyToEither / thenCombine on method-return CF.
  int useApplyToEitherId(BoxA ba, BoxB bb, CompletableFuture<A> oa, CompletableFuture<B> ob)
      throws Exception {
    return CompletableFuture.completedFuture(ba.get()).applyToEither(oa, x -> x).join().run()
        + CompletableFuture.completedFuture(bb.get()).applyToEither(ob, x -> x).join().run();
  }

  int useThenCombineId(BoxA ba, BoxB bb, CompletableFuture<A> oa, CompletableFuture<B> ob)
      throws Exception {
    return CompletableFuture.completedFuture(ba.get()).thenCombine(oa, (a, b) -> a).join().run()
        + CompletableFuture.completedFuture(bb.get()).thenCombine(ob, (a, b) -> a).join().run();
  }

  // Assigned var form.
  int useVar(BoxA ba, BoxB bb) throws Exception {
    var fa = CompletableFuture.completedFuture(0).thenApply(x -> ba.get());
    var fb = CompletableFuture.completedFuture(0).thenApply(x -> bb.get());
    var xa = Stream.of(0).map(x -> ba.get()).findFirst().get();
    var xb = Stream.of(0).map(x -> bb.get()).findFirst().get();
    return fa.join().run() + fb.join().run() + xa.run() + xb.run();
  }

  // Class regression — already worked.
  int useClass(CompletableFuture<Integer> o) throws Exception {
    return Stream.of(0).map(x -> new A()).findFirst().get().run()
        + Stream.of(0).map(x -> new B()).findFirst().get().run()
        + Optional.of(0).map(x -> new A()).get().run()
        + Optional.of(0).map(x -> new B()).get().run()
        + CompletableFuture.completedFuture(0).thenApply(x -> new A()).join().run()
        + CompletableFuture.completedFuture(0).thenApply(x -> new B()).join().run()
        + CompletableFuture.completedFuture(0).handle((x, e) -> new A()).join().run()
        + CompletableFuture.completedFuture(0).handle((x, e) -> new B()).join().run()
        + Stream.of(0).flatMap(x -> Stream.of(new A())).findFirst().get().run()
        + Stream.of(0).flatMap(x -> Stream.of(new B())).findFirst().get().run()
        + Optional.of(0).flatMap(x -> Optional.of(new A())).get().run()
        + Optional.of(0).flatMap(x -> Optional.of(new B())).get().run()
        + CompletableFuture.completedFuture(0)
            .thenCompose(x -> CompletableFuture.completedFuture(new A()))
            .join()
            .run()
        + CompletableFuture.completedFuture(0)
            .thenCompose(x -> CompletableFuture.completedFuture(new B()))
            .join()
            .run()
        + CompletableFuture.completedFuture(0).applyToEither(o, x -> new A()).join().run()
        + CompletableFuture.completedFuture(0).applyToEither(o, x -> new B()).join().run()
        + CompletableFuture.completedFuture(0).thenCombine(o, (a, b) -> new A()).join().run()
        + CompletableFuture.completedFuture(0).thenCombine(o, (a, b) -> new B()).join().run();
  }

  int usePreservesB(BoxB bb, CompletableFuture<Integer> o, CompletableFuture<B> ob)
      throws Exception {
    return Stream.of(0).map(x -> bb.get()).findFirst().get().run()
        + Optional.of(0).map(x -> bb.get()).get().run()
        + CompletableFuture.completedFuture(0).thenApply(x -> bb.get()).join().run()
        + CompletableFuture.completedFuture(0).handle((x, e) -> bb.get()).join().run()
        + Stream.of(0).flatMap(x -> Stream.of(bb.get())).findFirst().get().run()
        + Optional.of(0).flatMap(x -> Optional.of(bb.get())).get().run()
        + CompletableFuture.completedFuture(0)
            .thenCompose(x -> CompletableFuture.completedFuture(bb.get()))
            .join()
            .run()
        + CompletableFuture.completedFuture(0).applyToEither(o, x -> bb.get()).join().run()
        + CompletableFuture.completedFuture(0).thenCombine(o, (a, b) -> bb.get()).join().run()
        + CompletableFuture.completedFuture(bb.get()).applyToEither(ob, x -> x).join().run()
        + CompletableFuture.completedFuture(bb.get()).thenCombine(ob, (a, b) -> a).join().run()
        + Stream.of(0).map(x -> new B()).findFirst().get().run()
        + CompletableFuture.completedFuture(0).thenApply(x -> new B()).join().run();
  }
}
