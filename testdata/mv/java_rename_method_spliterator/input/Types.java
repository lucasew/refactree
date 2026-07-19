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
  public static int useSpliteratorTryAdvance(List<A> as, List<B> bs) {
    as.spliterator().tryAdvance(a -> a.run());
    bs.spliterator().tryAdvance(b -> b.run());
    return 0;
  }

  public static int useSpliteratorForEachRemaining(List<A> as, List<B> bs) {
    as.spliterator().forEachRemaining(a -> a.run());
    bs.spliterator().forEachRemaining(b -> b.run());
    return 0;
  }

  public static int useStreamSpliterator(List<A> as, List<B> bs) {
    as.stream().spliterator().tryAdvance(a -> a.run());
    bs.stream().spliterator().tryAdvance(b -> b.run());
    return 0;
  }

  public static int useVarSpliterator(List<A> as, List<B> bs) {
    var sa = as.spliterator();
    var sb = bs.spliterator();
    sa.tryAdvance(a -> a.run());
    sb.tryAdvance(b -> b.run());
    sa.forEachRemaining(a -> a.run());
    sb.forEachRemaining(b -> b.run());
    return 0;
  }

  public static int useTypedStill(List<A> as) {
    as.spliterator().tryAdvance((A a) -> a.run());
    return 0;
  }
}
