package demo;

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
  public static int useSubListGet(List<A> as, List<B> bs) {
    var xa = as.subList(0, 1).get(0);
    var xb = bs.subList(0, 1).get(0);
    return xa.run() + xb.run();
  }

  public static int useSubListGetFirst(List<A> as, List<B> bs) {
    var xa = as.subList(0, 1).getFirst();
    var xb = bs.subList(0, 1).getLast();
    return xa.run() + xb.run();
  }

  public static int useSubListForEach(List<A> as, List<B> bs) {
    as.subList(0, 1).forEach(a -> a.run());
    bs.subList(0, 1).forEach(b -> b.run());
    return 0;
  }

  public static int useSubListFor(List<A> as, List<B> bs) {
    int n = 0;
    for (var a : as.subList(0, 1)) {
      n += a.run();
    }
    for (var b : bs.subList(0, 1)) {
      n += b.run();
    }
    return n;
  }

  public static int useVarSubList(List<A> as, List<B> bs) {
    var al = as.subList(0, 1);
    var bl = bs.subList(0, 1);
    var xa = al.get(0);
    var xb = bl.get(0);
    al.forEach(a -> a.run());
    bl.forEach(b -> b.run());
    return xa.run() + xb.run();
  }

  public static int useSubListStream(List<A> as, List<B> bs) {
    as.subList(0, 1).stream().forEach(a -> a.run());
    bs.subList(0, 1).stream().forEach(b -> b.run());
    return 0;
  }
}
