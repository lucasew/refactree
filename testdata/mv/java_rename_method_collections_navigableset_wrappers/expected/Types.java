package demo;

import java.util.Collections;
import java.util.NavigableSet;
import java.util.SortedSet;

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
  // Chain: Collections NavigableSet/SortedSet wrappers are element-type-preserving
  // (same path as unmodifiableSet / unmodifiableNavigableMap; Class args ignored).
  public static int useUnmodifiableNavigableSetFirst(
      NavigableSet<A> as, NavigableSet<B> bs) {
    return Collections.unmodifiableNavigableSet(as).first().execute()
        + Collections.unmodifiableNavigableSet(bs).last().run();
  }

  public static int useSynchronizedNavigableSetFirst(
      NavigableSet<A> as, NavigableSet<B> bs) {
    return Collections.synchronizedNavigableSet(as).first().execute()
        + Collections.synchronizedNavigableSet(bs).last().run();
  }

  public static int useCheckedNavigableSetFirst(
      NavigableSet<A> as, NavigableSet<B> bs) {
    return Collections.checkedNavigableSet(as, A.class).first().execute()
        + Collections.checkedNavigableSet(bs, B.class).last().run();
  }

  public static int useUnmodifiableSortedSetFirst(SortedSet<A> as, SortedSet<B> bs) {
    return Collections.unmodifiableSortedSet(as).first().execute()
        + Collections.unmodifiableSortedSet(bs).last().run();
  }

  public static int useSynchronizedSortedSetFirst(SortedSet<A> as, SortedSet<B> bs) {
    return Collections.synchronizedSortedSet(as).first().execute()
        + Collections.synchronizedSortedSet(bs).last().run();
  }

  public static int useCheckedSortedSetFirst(SortedSet<A> as, SortedSet<B> bs) {
    return Collections.checkedSortedSet(as, A.class).first().execute()
        + Collections.checkedSortedSet(bs, B.class).last().run();
  }

  // var from wrapper then first — element leaf through elemOf.
  public static int useVarWrapperFirst(NavigableSet<A> as, NavigableSet<B> bs) {
    var al = Collections.unmodifiableNavigableSet(as);
    var bl = Collections.synchronizedNavigableSet(bs);
    var cl = Collections.checkedNavigableSet(as, A.class);
    var xa = al.first();
    var xb = bl.last();
    var xc = cl.first();
    return xa.execute() + xb.run() + xc.execute();
  }

  public static int useVarSortedFirst(SortedSet<A> as, SortedSet<B> bs) {
    var al = Collections.unmodifiableSortedSet(as);
    var bl = Collections.synchronizedSortedSet(bs);
    return al.first().execute() + bl.last().run();
  }

  // ceiling/floor through wrapper (same E; search only).
  public static int useCeilingThroughWrapper(NavigableSet<A> as, NavigableSet<B> bs) {
    return Collections.unmodifiableNavigableSet(as).ceiling(new A()).execute()
        + Collections.unmodifiableNavigableSet(bs).floor(new B()).run();
  }

  // forEach / for-in through wrapper (neighbor paths).
  public static int useWrapperForEach(NavigableSet<A> as, NavigableSet<B> bs) {
    Collections.unmodifiableNavigableSet(as).forEach(a -> a.execute());
    Collections.unmodifiableNavigableSet(bs).forEach(b -> b.run());
    return 0;
  }

  public static int useWrapperForIn(SortedSet<A> as, SortedSet<B> bs) {
    int n = 0;
    for (var a : Collections.unmodifiableSortedSet(as)) {
      n += a.execute();
    }
    for (var b : Collections.unmodifiableSortedSet(bs)) {
      n += b.run();
    }
    return n;
  }

  // Regression: plain unmodifiableSet / bare first already worked.
  public static int usePlainUnmodifiableSet(NavigableSet<A> as, NavigableSet<B> bs) {
    Collections.unmodifiableSet(as).forEach(a -> a.execute());
    Collections.unmodifiableSet(bs).forEach(b -> b.run());
    return 0;
  }

  public static int usePlainFirst(NavigableSet<A> as, NavigableSet<B> bs) {
    var xa = as.first();
    var xb = bs.last();
    return xa.execute() + xb.run();
  }
}
