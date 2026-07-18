package demo;

import java.util.NavigableSet;
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
  public static int useCeiling(NavigableSet<A> as, NavigableSet<B> bs, A probeA, B probeB) {
    var xa = as.ceiling(probeA);
    var xb = bs.ceiling(probeB);
    return xa.run() + xb.run();
  }

  public static int useFloor(NavigableSet<A> as, NavigableSet<B> bs, A probeA, B probeB) {
    var fa = as.floor(probeA);
    var fb = bs.floor(probeB);
    return fa.run() + fb.run();
  }

  public static int useHigherLower(NavigableSet<A> as, NavigableSet<B> bs, A probeA, B probeB) {
    var ha = as.higher(probeA);
    var lb = bs.lower(probeB);
    return ha.run() + lb.run();
  }

  public static int useTreeSet(TreeSet<A> as, TreeSet<B> bs, A probeA, B probeB) {
    var xa = as.ceiling(probeA);
    var xb = bs.floor(probeB);
    var ya = as.higher(probeA);
    var yb = bs.lower(probeB);
    return xa.run() + xb.run() + ya.run() + yb.run();
  }
}
