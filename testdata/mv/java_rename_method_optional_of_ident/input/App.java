import java.util.Optional;

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

class App {
  int useOfNullable(A a, B b) {
    return Optional.ofNullable(a).get().run() + Optional.ofNullable(b).get().run();
  }

  int useOf(A a, B b) {
    return Optional.of(a).get().run() + Optional.of(b).get().run();
  }

  int useOrElseThrow(A a, B b) {
    return Optional.ofNullable(a).orElseThrow().run()
        + Optional.ofNullable(b).orElseThrow().run();
  }

  int useAssign(A a, B b) {
    A xa = Optional.ofNullable(a).get();
    B xb = Optional.ofNullable(b).get();
    return xa.run() + xb.run();
  }
}
