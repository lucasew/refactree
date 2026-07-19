package demo;

import java.util.List;
import java.util.ListIterator;

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
  public static int useListIteratorNext(List<A> as, List<B> bs) {
    var xa = as.listIterator().next();
    var xb = bs.listIterator().next();
    return xa.run() + xb.run();
  }

  public static int useListIteratorPrevious(List<A> as, List<B> bs) {
    var xa = as.listIterator().previous();
    var xb = bs.listIterator().previous();
    return xa.run() + xb.run();
  }

  public static int useTypedListIterator(List<A> as, List<B> bs) {
    ListIterator<A> ia = as.listIterator();
    ListIterator<B> ib = bs.listIterator();
    var xa = ia.previous();
    var xb = ib.previous();
    return xa.run() + xb.run();
  }

  public static int useTypedListIteratorNext(List<A> as, List<B> bs) {
    ListIterator<A> ia = as.listIterator();
    ListIterator<B> ib = bs.listIterator();
    var xa = ia.next();
    var xb = ib.next();
    return xa.run() + xb.run();
  }
}
