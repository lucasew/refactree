package demo;

import java.util.List;
import java.util.stream.Collectors;

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
  public static int useTeeingList(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.teeing(Collectors.toList(), Collectors.counting(), (xs, n) -> xs)).forEach(a -> a.execute());
    bs.stream().collect(Collectors.teeing(Collectors.toList(), Collectors.counting(), (xs, n) -> xs)).forEach(b -> b.run());
    return 0;
  }

  public static int useTeeingSecondList(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.teeing(Collectors.counting(), Collectors.toList(), (n, xs) -> xs)).forEach(a -> a.execute());
    bs.stream().collect(Collectors.teeing(Collectors.counting(), Collectors.toList(), (n, xs) -> xs)).forEach(b -> b.run());
    return 0;
  }

  public static int useTeeingToSet(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.teeing(Collectors.toSet(), Collectors.counting(), (xs, n) -> xs)).forEach(a -> a.execute());
    bs.stream().collect(Collectors.teeing(Collectors.toSet(), Collectors.counting(), (xs, n) -> xs)).forEach(b -> b.run());
    return 0;
  }

  public static int useVar(List<A> as, List<B> bs) {
    var al = as.stream().collect(Collectors.teeing(Collectors.toList(), Collectors.counting(), (xs, n) -> xs));
    var bl = bs.stream().collect(Collectors.teeing(Collectors.toList(), Collectors.counting(), (xs, n) -> xs));
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
}
