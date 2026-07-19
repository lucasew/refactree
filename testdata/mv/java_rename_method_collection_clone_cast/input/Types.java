package demo;

import java.util.ArrayList;

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
  public static int useCloneCast(ArrayList<A> as, ArrayList<B> bs) {
    return ((ArrayList<A>) as.clone()).get(0).run() + ((ArrayList<B>) bs.clone()).get(0).run();
  }

  public static int useCloneForEach(ArrayList<A> as, ArrayList<B> bs) {
    ((ArrayList<A>) as.clone()).forEach(a -> a.run());
    ((ArrayList<B>) bs.clone()).forEach(b -> b.run());
    return 0;
  }

  public static int useCloneVar(ArrayList<A> as, ArrayList<B> bs) {
    var al = (ArrayList<A>) as.clone();
    var bl = (ArrayList<B>) bs.clone();
    return al.get(0).run() + bl.get(0).run();
  }

  public static int usePreservesB(ArrayList<B> bs) {
    var xb = ((ArrayList<B>) bs.clone()).get(0);
    return ((ArrayList<B>) bs.clone()).get(0).run() + xb.run();
  }
}
