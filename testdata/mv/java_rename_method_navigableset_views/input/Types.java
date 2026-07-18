package demo;

import java.util.NavigableSet;
import java.util.SortedSet;
import java.util.TreeSet;

public class A {
  public int run() {
    return 1;
  }
}

class B {
  public int run() {
    return 2;
  }
}

class Uses {
  // Chain: descendingSet is element-type-preserving like List.subList / reversed.
  public static int useDescendingSetFirst(NavigableSet<A> as, NavigableSet<B> bs) {
    return as.descendingSet().first().run() + bs.descendingSet().first().run();
  }

  public static int useHeadSetFirst(SortedSet<A> as, SortedSet<B> bs) {
    return as.headSet(new A()).first().run() + bs.headSet(new B()).first().run();
  }

  public static int useTailSetLast(SortedSet<A> as, SortedSet<B> bs) {
    return as.tailSet(new A()).last().run() + bs.tailSet(new B()).last().run();
  }

  public static int useSubSetFirst(NavigableSet<A> as, NavigableSet<B> bs) {
    return as.subSet(new A(), new A()).first().run()
        + bs.subSet(new B(), new B()).first().run();
  }

  // NavigableSet overloads with inclusivity flags (bounds only; same E).
  public static int useInclusiveViews(NavigableSet<A> as, NavigableSet<B> bs) {
    return as.headSet(new A(), true).first().run()
        + bs.tailSet(new B(), false).last().run()
        + as.subSet(new A(), true, new A(), false).first().run();
  }

  // var from view then first — element leaf through elemOf (unique names).
  public static int useVarDescending(NavigableSet<A> as, NavigableSet<B> bs) {
    var daView = as.descendingSet();
    var dbView = bs.descendingSet();
    var xaView = daView.first();
    var xbView = dbView.first();
    return xaView.run() + xbView.run();
  }

  public static int useVarHeadTailSub(SortedSet<A> as, SortedSet<B> bs) {
    var haView = as.headSet(new A());
    var tbView = bs.tailSet(new B());
    var saView = as.subSet(new A(), new A());
    return haView.first().run() + tbView.last().run() + saView.first().run();
  }

  // forEach / for-in through view.
  public static int useDescendingForEach(NavigableSet<A> as, NavigableSet<B> bs) {
    as.descendingSet().forEach(a -> a.run());
    bs.descendingSet().forEach(b -> b.run());
    return 0;
  }

  public static int useHeadSetForIn(TreeSet<A> as, TreeSet<B> bs) {
    int n = 0;
    for (var a : as.headSet(new A())) {
      n += a.run();
    }
    for (var b : bs.tailSet(new B())) {
      n += b.run();
    }
    return n;
  }

  public static int useSubSetStream(NavigableSet<A> as, NavigableSet<B> bs) {
    as.subSet(new A(), new A()).stream().forEach(a -> a.run());
    bs.subSet(new B(), new B()).stream().forEach(b -> b.run());
    return 0;
  }

  // Regression: plain first / ceiling already worked.
  public static int usePlainFirst(NavigableSet<A> as, NavigableSet<B> bs) {
    var xa = as.first();
    var xb = bs.first();
    return xa.run() + xb.run();
  }
}
