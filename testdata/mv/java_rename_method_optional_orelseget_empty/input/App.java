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
  int useOrElseGet() {
    return Optional.<A>empty().orElseGet(() -> new A()).run()
        + Optional.<B>empty().orElseGet(() -> new B()).run();
  }

  int useOrElse() {
    return Optional.<A>empty().orElse(new A()).run()
        + Optional.<B>empty().orElse(new B()).run();
  }

  int useOrElseGetAssign() {
    A xa = Optional.<A>empty().orElseGet(() -> new A());
    B xb = Optional.<B>empty().orElseGet(() -> new B());
    return xa.run() + xb.run();
  }
}
