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
  public static int useStream(List<A> as) {
    return as.stream().map(a -> a.execute()).mapToInt(i -> i).sum();
  }

  public static int useStreamB(List<B> bs) {
    return bs.stream().map(b -> b.run()).mapToInt(i -> i).sum();
  }

  public static int useMapToInt(List<A> as, List<B> bs) {
    int x = as.stream().mapToInt(a -> a.execute()).sum();
    int y = bs.stream().mapToInt(b -> b.run()).sum();
    return x + y;
  }

  public static int useForEach(List<A> as, List<B> bs) {
    as.forEach(a -> a.execute());
    bs.forEach(b -> b.run());
    return 0;
  }

  public static int useFilter(List<A> as) {
    return as.stream().filter(a -> a.execute() > 0).mapToInt(a -> a.execute()).sum();
  }

  public static int useTypedStill(List<A> as) {
    return as.stream().map((A a) -> a.execute()).mapToInt(i -> i).sum();
  }
}
