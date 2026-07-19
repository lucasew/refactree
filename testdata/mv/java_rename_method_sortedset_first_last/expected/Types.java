package demo;

import java.util.NavigableSet;
import java.util.SortedSet;
import java.util.TreeSet;

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
  public static int useFirst(SortedSet<A> as, SortedSet<B> bs) {
    var xa = as.first();
    var xb = bs.first();
    return xa.execute() + xb.run();
  }

  public static int useLast(SortedSet<A> as, SortedSet<B> bs) {
    var ya = as.last();
    var yb = bs.last();
    return ya.execute() + yb.run();
  }

  public static int useNavigable(NavigableSet<A> as, NavigableSet<B> bs) {
    var za = as.first();
    var zb = bs.last();
    return za.execute() + zb.run();
  }

  public static int useTreeSet(TreeSet<A> as, TreeSet<B> bs) {
    var xa = as.first();
    var xb = bs.last();
    return xa.execute() + xb.run();
  }
}
