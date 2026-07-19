package demo;

import java.util.Collections;
import java.util.Enumeration;
import java.util.List;
import java.util.Vector;

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
  public static int useEnumerationNextElement(List<A> as, List<B> bs) {
    var xa = Collections.enumeration(as).nextElement();
    var xb = Collections.enumeration(bs).nextElement();
    return xa.execute() + xb.run();
  }

  public static int useTypedEnumeration(Enumeration<A> ea, Enumeration<B> eb) {
    var xa = ea.nextElement();
    var xb = eb.nextElement();
    return xa.execute() + xb.run();
  }

  public static int useVectorElements(Vector<A> as, Vector<B> bs) {
    var xa = as.elements().nextElement();
    var xb = bs.elements().nextElement();
    return xa.execute() + xb.run();
  }
}
