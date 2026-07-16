package demo;

import java.util.Iterator;
import java.util.List;
import java.util.Map;

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
  public static int useListGet(List<A> as, List<B> bs) {
    var xa = as.get(0);
    var xb = bs.get(0);
    return xa.execute() + xb.run();
  }

  public static int useMapGet(Map<String, A> am, Map<String, B> bm) {
    var xa = am.get("k");
    var xb = bm.get("k");
    return xa.execute() + xb.run();
  }

  public static int useIteratorNext(List<A> as, List<B> bs) {
    var xa = as.iterator().next();
    var xb = bs.iterator().next();
    return xa.execute() + xb.run();
  }

  public static int useTypedIterator(List<A> as, List<B> bs) {
    Iterator<A> ia = as.iterator();
    Iterator<B> ib = bs.iterator();
    var xa = ia.next();
    var xb = ib.next();
    return xa.execute() + xb.run();
  }
}
