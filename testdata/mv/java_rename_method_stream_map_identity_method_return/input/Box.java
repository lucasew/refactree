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
  int useMapId(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).map(x -> x).findFirst().get().run()
        + Stream.of(bb.get()).map(x -> x).findFirst().get().run();
  }

  int useMapIdAssign(BoxA ba, BoxB bb) {
    var xa = Stream.of(ba.get()).map(x -> x).findFirst().get();
    var xb = Stream.of(bb.get()).map(x -> x).findFirst().get();
    return xa.run() + xb.run();
  }

  int useDirect(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).findFirst().get().run()
        + Stream.of(bb.get()).findFirst().get().run();
  }

  int useClass() {
    return Stream.of(new A()).map(x -> x).findFirst().get().run()
        + Stream.of(new B()).map(x -> x).findFirst().get().run();
  }

  int usePreservesB(BoxB bb) {
    return Stream.of(bb.get()).map(x -> x).findFirst().get().run();
  }
}
