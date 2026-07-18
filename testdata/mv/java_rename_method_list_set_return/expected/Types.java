package demo;

import java.util.ArrayList;
import java.util.List;

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
  public static int useSet(List<A> as, List<B> bs) {
    var xa = as.set(0, new A());
    var xb = bs.set(0, new B());
    return xa.execute() + xb.run();
  }

  public static int useArrayList(ArrayList<A> as, ArrayList<B> bs) {
    var ya = as.set(0, new A());
    var yb = bs.set(0, new B());
    return ya.execute() + yb.run();
  }
}
