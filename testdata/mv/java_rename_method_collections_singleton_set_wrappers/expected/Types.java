package demo;

import java.util.Collection;
import java.util.Collections;
import java.util.Set;

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
  public static int useSingletonForEach() {
    Collections.singleton(new A()).forEach(a -> a.execute());
    Collections.singleton(new B()).forEach(b -> b.run());
    return 0;
  }

  public static int useVarSingleton() {
    var as = Collections.singleton(new A());
    var bs = Collections.singleton(new B());
    as.forEach(a -> a.execute());
    bs.forEach(b -> b.run());
    int n = 0;
    for (var a : as) {
      n += a.execute();
    }
    for (var b : bs) {
      n += b.run();
    }
    return n;
  }

  public static int useSingletonFor() {
    int n = 0;
    for (var a : Collections.singleton(new A())) {
      n += a.execute();
    }
    for (var b : Collections.singleton(new B())) {
      n += b.run();
    }
    return n;
  }

  public static int useUnmodifiableSetForEach(Set<A> as, Set<B> bs) {
    Collections.unmodifiableSet(as).forEach(a -> a.execute());
    Collections.unmodifiableSet(bs).forEach(b -> b.run());
    return 0;
  }

  public static int useSynchronizedSetForEach(Set<A> as, Set<B> bs) {
    Collections.synchronizedSet(as).forEach(a -> a.execute());
    Collections.synchronizedSet(bs).forEach(b -> b.run());
    return 0;
  }

  public static int useCheckedSetForEach(Set<A> as, Set<B> bs) {
    Collections.checkedSet(as, A.class).forEach(a -> a.execute());
    Collections.checkedSet(bs, B.class).forEach(b -> b.run());
    return 0;
  }

  public static int useUnmodifiableCollectionForEach(Collection<A> as, Collection<B> bs) {
    Collections.unmodifiableCollection(as).forEach(a -> a.execute());
    Collections.unmodifiableCollection(bs).forEach(b -> b.run());
    return 0;
  }

  public static int useVarSetWrappers(Set<A> as, Set<B> bs) {
    var al = Collections.unmodifiableSet(as);
    var bl = Collections.synchronizedSet(bs);
    var cl = Collections.checkedSet(as, A.class);
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

  public static int useWrapperFor(Set<A> as, Set<B> bs) {
    int n = 0;
    for (var a : Collections.unmodifiableSet(as)) {
      n += a.execute();
    }
    for (var b : Collections.checkedSet(bs, B.class)) {
      n += b.run();
    }
    return n;
  }
}
