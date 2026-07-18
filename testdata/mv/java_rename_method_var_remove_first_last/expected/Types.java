package demo;

import java.util.Deque;
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
  public static int useRemoveFirst(List<A> as, List<B> bs) {
    var xa = as.removeFirst();
    var xb = bs.removeFirst();
    return xa.execute() + xb.run();
  }

  public static int useRemoveLast(List<A> as, List<B> bs) {
    var ya = as.removeLast();
    var yb = bs.removeLast();
    return ya.execute() + yb.run();
  }

  public static int useDequeRemove(Deque<A> da, Deque<B> db) {
    var xa = da.removeFirst();
    var xb = db.removeLast();
    return xa.execute() + xb.run();
  }
}
