import java.util.List;
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
  // Stream.of(method-return).toArray(T[]::new)[0] under foreign same-leaf.
  int useStreamToArrayGen(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).toArray(A[]::new)[0].execute()
        + Stream.of(bb.get()).toArray(B[]::new)[0].run();
  }

  int useStreamToArrayNew(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).toArray(new A[0])[0].execute()
        + Stream.of(bb.get()).toArray(new B[0])[0].run();
  }

  int useListToArray(BoxA ba, BoxB bb) {
    return List.of(ba.get()).toArray(A[]::new)[0].execute()
        + List.of(bb.get()).toArray(B[]::new)[0].run();
  }

  int useCollectToArray(BoxA ba, BoxB bb) {
    return Stream.of(ba.get()).collect(Collectors.toList()).toArray(A[]::new)[0].execute()
        + Stream.of(bb.get()).collect(Collectors.toList()).toArray(B[]::new)[0].run();
  }

  int useVar(BoxA ba, BoxB bb) {
    var aa = Stream.of(ba.get()).toArray(A[]::new);
    var ab = Stream.of(bb.get()).toArray(B[]::new);
    return aa[0].execute() + ab[0].run();
  }

  // Class regression — already worked.
  int useClass() {
    return Stream.of(new A()).toArray(A[]::new)[0].execute()
        + Stream.of(new B()).toArray(B[]::new)[0].run()
        + List.of(new A()).toArray(A[]::new)[0].execute()
        + List.of(new B()).toArray(B[]::new)[0].run()
        + Stream.of(new A()).toArray(new A[0])[0].execute()
        + Stream.of(new B()).toArray(new B[0])[0].run();
  }

  int usePreservesB(BoxB bb) {
    return Stream.of(bb.get()).toArray(B[]::new)[0].run()
        + Stream.of(new B()).toArray(B[]::new)[0].run()
        + List.of(bb.get()).toArray(B[]::new)[0].run()
        + List.of(new B()).toArray(B[]::new)[0].run();
  }
}
