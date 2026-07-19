import java.util.Optional;
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
  int useStreamFlatMap(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).flatMap(x -> Stream.of(x)).findFirst().get().execute()
        + Stream.of(bb.get()).flatMap(x -> Stream.of(x)).findFirst().get().run();
  }

  int useStreamFlatMapAssign(BoxA ba, BoxB bb) {
    var xa = Stream.of(ba.get()).flatMap(x -> Stream.of(x)).findFirst().get();
    var xb = Stream.of(bb.get()).flatMap(x -> Stream.of(x)).findFirst().get();
    return xa.execute() + xb.run();
  }

  int useOptFlatMap(BoxA ba, BoxB bb) {
    return Optional.of(ba.get()).flatMap(x -> Optional.of(x)).get().execute()
        + Optional.of(bb.get()).flatMap(x -> Optional.of(x)).get().run();
  }

  int useOptFlatMapAssign(BoxA ba, BoxB bb) {
    var xa = Optional.of(ba.get()).flatMap(x -> Optional.of(x)).get();
    var xb = Optional.of(bb.get()).flatMap(x -> Optional.of(x)).get();
    return xa.execute() + xb.run();
  }

  int useClass() {
    return Stream.of(new A()).flatMap(x -> Stream.of(x)).findFirst().get().execute()
        + Stream.of(new B()).flatMap(x -> Stream.of(x)).findFirst().get().run()
        + Optional.of(new A()).flatMap(x -> Optional.of(x)).get().execute()
        + Optional.of(new B()).flatMap(x -> Optional.of(x)).get().run();
  }

  int usePreservesB(BoxB bb) {
    return Stream.of(bb.get()).flatMap(x -> Stream.of(x)).findFirst().get().run()
        + Optional.of(bb.get()).flatMap(x -> Optional.of(x)).get().run();
  }

  int useDirect(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).findFirst().get().execute()
        + Optional.of(ba.get()).get().execute()
        + Stream.of(bb.get()).findFirst().get().run()
        + Optional.of(bb.get()).get().run();
  }
}
