package demo;

import java.util.Iterator;
import java.util.List;

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
  public static int useIter(List<A> as, List<B> bs) {
    as.iterator().forEachRemaining(a -> a.run());
    bs.iterator().forEachRemaining(b -> b.run());
    return 0;
  }

  public static int useStreamIter(List<A> as, List<B> bs) {
    as.stream().iterator().forEachRemaining(a -> a.run());
    bs.stream().iterator().forEachRemaining(b -> b.run());
    return 0;
  }

  public static int useTypedStill(List<A> as) {
    as.iterator().forEachRemaining((A a) -> a.run());
    return 0;
  }
}
