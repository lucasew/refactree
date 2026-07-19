package demo;

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
  public static int useReversedForEach(List<A> as, List<B> bs) {
    as.reversed().forEach(a -> a.execute());
    bs.reversed().forEach(b -> b.run());
    return 0;
  }

  public static int useReversedFor(List<A> as, List<B> bs) {
    int n = 0;
    for (var a : as.reversed()) {
      n += a.execute();
    }
    for (var b : bs.reversed()) {
      n += b.run();
    }
    return n;
  }

  public static int useVarReversed(List<A> as, List<B> bs) {
    var al = as.reversed();
    var bl = bs.reversed();
    al.forEach(a -> a.execute());
    bl.forEach(b -> b.run());
    int n = 0;
    for (var a : al) {
      n += a.execute();
    }
    for (var b : bl) {
      n += b.run();
    }
    return n;
  }

  public static int useReversedStream(List<A> as, List<B> bs) {
    as.reversed().stream().forEach(a -> a.execute());
    bs.reversed().stream().forEach(b -> b.run());
    return 0;
  }
}
