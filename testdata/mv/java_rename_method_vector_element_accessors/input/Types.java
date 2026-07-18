package demo;

import java.util.Vector;

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
  public static int useElementAt(Vector<A> as, Vector<B> bs) {
    var xa = as.elementAt(0);
    var xb = bs.elementAt(0);
    return xa.run() + xb.run();
  }

  public static int useFirstElement(Vector<A> as, Vector<B> bs) {
    var ya = as.firstElement();
    var yb = bs.firstElement();
    return ya.run() + yb.run();
  }

  public static int useLastElement(Vector<A> as, Vector<B> bs) {
    var za = as.lastElement();
    var zb = bs.lastElement();
    return za.run() + zb.run();
  }
}
