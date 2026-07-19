import java.util.Arrays;
import java.util.Collections;
import java.util.List;
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
  // Collections.nCopies(method-return) peels under foreign same-leaf.
  int useNCopiesGet() {
    return Collections.nCopies(1, new BoxA().get()).get(0).execute()
        + Collections.nCopies(1, new BoxB().get()).get(0).run();
  }

  int useNCopiesIterator() {
    return Collections.nCopies(1, new BoxA().get()).iterator().next().execute()
        + Collections.nCopies(1, new BoxB().get()).iterator().next().run();
  }

  int useNCopiesStream() {
    return Collections.nCopies(1, new BoxA().get()).stream().findFirst().get().execute()
        + Collections.nCopies(1, new BoxB().get()).stream().findFirst().get().run();
  }

  int useNCopiesForEach() {
    int[] n = {0};
    Collections.nCopies(1, new BoxA().get()).forEach(xa -> n[0] += xa.execute());
    Collections.nCopies(1, new BoxB().get()).forEach(xb -> n[0] += xb.run());
    return n[0];
  }

  int useNCopiesEnhancedFor() {
    int n = 0;
    for (var xa : Collections.nCopies(1, new BoxA().get())) {
      n += xa.execute();
    }
    for (var xb : Collections.nCopies(1, new BoxB().get())) {
      n += xb.run();
    }
    return n;
  }

  int useNCopiesVar() {
    var al = Collections.nCopies(1, new BoxA().get());
    var bl = Collections.nCopies(1, new BoxB().get());
    return al.get(0).execute()
        + bl.get(0).run()
        + al.iterator().next().execute()
        + bl.iterator().next().run();
  }

  // List/Stream/Arrays factory forEach method-return (Class forEach already solid).
  int useListOfForEach() {
    int[] n = {0};
    List.of(new BoxA().get()).forEach(xa -> n[0] += xa.execute());
    List.of(new BoxB().get()).forEach(xb -> n[0] += xb.run());
    return n[0];
  }

  int useStreamOfForEach() {
    int[] n = {0};
    Stream.of(new BoxA().get()).forEach(xa -> n[0] += xa.execute());
    Stream.of(new BoxB().get()).forEach(xb -> n[0] += xb.run());
    return n[0];
  }

  int useAsListSingletonForEach() {
    int[] n = {0};
    Arrays.asList(new BoxA().get()).forEach(xa -> n[0] += xa.execute());
    Arrays.asList(new BoxB().get()).forEach(xb -> n[0] += xb.run());
    Collections.singletonList(new BoxA().get()).forEach(xa -> n[0] += xa.execute());
    Collections.singletonList(new BoxB().get()).forEach(xb -> n[0] += xb.run());
    return n[0];
  }

  int useListOfEnhancedFor() {
    int n = 0;
    for (var xa : List.of(new BoxA().get())) {
      n += xa.execute();
    }
    for (var xb : List.of(new BoxB().get())) {
      n += xb.run();
    }
    return n;
  }

  // Class() forms already solid — keep as regression.
  int useClassNCopies() {
    return Collections.nCopies(1, new A()).iterator().next().execute()
        + Collections.nCopies(1, new B()).iterator().next().run()
        + Collections.nCopies(1, new A()).get(0).execute()
        + Collections.nCopies(1, new B()).get(0).run();
  }

  int useClassListForEach() {
    int[] n = {0};
    List.of(new A()).forEach(xa -> n[0] += xa.execute());
    List.of(new B()).forEach(xb -> n[0] += xb.run());
    Collections.nCopies(1, new A()).forEach(xa -> n[0] += xa.execute());
    Collections.nCopies(1, new B()).forEach(xb -> n[0] += xb.run());
    return n[0];
  }
}
