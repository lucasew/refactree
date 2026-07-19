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
  // mapMulti bi-lambda first param is stream element T.
  public static int useMapMultiBody(List<A> as, List<B> bs) {
    as.stream().mapMulti((a, c) -> c.accept(a.run())).forEach(i -> {});
    bs.stream().mapMulti((b, c) -> c.accept(b.run())).forEach(i -> {});
    return 0;
  }

  // Identity mapMulti peels T for downstream forEach.
  public static int useMapMultiForEach(List<A> as, List<B> bs) {
    as.stream().mapMulti((a, c) -> c.accept(a)).forEach(x -> x.run());
    bs.stream().mapMulti((b, c) -> c.accept(b)).forEach(y -> y.run());
    return 0;
  }

  // mapMulti to new T peels construction type for forEach.
  public static int useMapMultiNew(List<A> as, List<B> bs) {
    as.stream().mapMulti((a, c) -> c.accept(new A())).forEach(x -> x.run());
    bs.stream().mapMulti((b, c) -> c.accept(new B())).forEach(y -> y.run());
    return 0;
  }

  public static int useVarMapMulti(List<A> as, List<B> bs) {
    var sa = as.stream().mapMulti((a, c) -> c.accept(a));
    var sb = bs.stream().mapMulti((b, c) -> c.accept(b));
    sa.forEach(x -> x.run());
    sb.forEach(y -> y.run());
    return 0;
  }

  public static int usePreservesB(List<B> bs) {
    bs.stream().mapMulti((b, c) -> c.accept(b)).forEach(y -> y.run());
    return 0;
  }
}
