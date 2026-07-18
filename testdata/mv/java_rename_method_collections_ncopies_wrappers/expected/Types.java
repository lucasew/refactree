package demo;

import java.util.Collections;
import java.util.List;

public class A {
  public int execute() {
    return 1;
  }
}

class B {
  public int run() {
    return 2;
  }
}

class Uses {
  public static int useNCopiesForEach() {
    Collections.nCopies(2, new A()).forEach(a -> a.execute());
    Collections.nCopies(2, new B()).forEach(b -> b.run());
    return 0;
  }

  public static int useVarNCopies() {
    var al = Collections.nCopies(2, new A());
    var bl = Collections.nCopies(2, new B());
    al.forEach(a -> a.execute());
    bl.forEach(b -> b.run());
    int n = 0;
    for (var a : al) {
      n += a.execute();
    }
    for (var b : bl) {
      n += b.run();
    }
    return n;
  }

  public static int useNCopiesFor() {
    int n = 0;
    for (var a : Collections.nCopies(2, new A())) {
      n += a.execute();
    }
    for (var b : Collections.nCopies(2, new B())) {
      n += b.run();
    }
    return n;
  }

  public static int useUnmodifiableForEach(List<A> as, List<B> bs) {
    Collections.unmodifiableList(as).forEach(a -> a.execute());
    Collections.unmodifiableList(bs).forEach(b -> b.run());
    return 0;
  }

  public static int useSynchronizedForEach(List<A> as, List<B> bs) {
    Collections.synchronizedList(as).forEach(a -> a.execute());
    Collections.synchronizedList(bs).forEach(b -> b.run());
    return 0;
  }

  public static int useCheckedForEach(List<A> as, List<B> bs) {
    Collections.checkedList(as, A.class).forEach(a -> a.execute());
    Collections.checkedList(bs, B.class).forEach(b -> b.run());
    return 0;
  }

  public static int useVarWrappers(List<A> as, List<B> bs) {
    var al = Collections.unmodifiableList(as);
    var bl = Collections.synchronizedList(bs);
    var cl = Collections.checkedList(as, A.class);
    al.forEach(a -> a.execute());
    bl.forEach(b -> b.run());
    cl.forEach(a -> a.execute());
    int n = 0;
    for (var a : al) {
      n += a.execute();
    }
    for (var b : bl) {
      n += b.run();
    }
    for (var a : cl) {
      n += a.execute();
    }
    return n;
  }

  public static int useWrapperFor(List<A> as, List<B> bs) {
    int n = 0;
    for (var a : Collections.unmodifiableList(as)) {
      n += a.execute();
    }
    for (var b : Collections.checkedList(bs, B.class)) {
      n += b.run();
    }
    return n;
  }
}
