import java.util.List;
import java.util.Collections;
import java.util.stream.Stream;
import java.util.stream.Collectors;

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
  int useCollectToList(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).collect(Collectors.toList()).get(0).execute()
      + Stream.of(bb.get()).collect(Collectors.toList()).get(0).run();
  }

  int useUnmodifiable(BoxA ba, BoxB bb) {
    return Collections.unmodifiableList(List.of(ba.get())).get(0).execute()
      + Collections.unmodifiableList(List.of(bb.get())).get(0).run();
  }

  int useUnmodifiableAssign(BoxA ba, BoxB bb) {
    var xs = Collections.unmodifiableList(List.of(ba.get()));
    var ys = Collections.unmodifiableList(List.of(bb.get()));
    return xs.get(0).execute() + ys.get(0).run();
  }

  int useCollectAssign(BoxA ba, BoxB bb) {
    var xs = Stream.of(ba.get()).collect(Collectors.toList());
    var ys = Stream.of(bb.get()).collect(Collectors.toList());
    return xs.get(0).execute() + ys.get(0).run();
  }

  int useClass() {
    return Stream.of(new A()).collect(Collectors.toList()).get(0).execute()
      + Stream.of(new B()).collect(Collectors.toList()).get(0).run()
      + Collections.unmodifiableList(List.of(new A())).get(0).execute()
      + Collections.unmodifiableList(List.of(new B())).get(0).run();
  }

  int usePreservesB(BoxB bb) {
    return Stream.of(bb.get()).collect(Collectors.toList()).get(0).run()
      + Collections.unmodifiableList(List.of(bb.get())).get(0).run();
  }
}
