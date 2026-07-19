package demo;

import java.util.List;
import java.util.stream.Collectors;

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
  public static int useToSetForEach(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.toSet()).forEach(a -> a.run());
    bs.stream().collect(Collectors.toSet()).forEach(b -> b.run());
    return 0;
  }

  public static int useToSetFor(List<A> as, List<B> bs) {
    int n = 0;
    for (var a : as.stream().collect(Collectors.toSet())) {
      n += a.run();
    }
    for (var b : bs.stream().collect(Collectors.toSet())) {
      n += b.run();
    }
    return n;
  }

  public static int useVarToSet(List<A> as, List<B> bs) {
    var al = as.stream().collect(Collectors.toSet());
    var bl = bs.stream().collect(Collectors.toSet());
    al.forEach(a -> a.run());
    bl.forEach(b -> b.run());
    return 0;
  }

  public static int useMethodRef(List<A> as, List<B> bs) {
    as.stream().collect(Collectors::toSet).forEach(a -> a.run());
    bs.stream().collect(Collectors::toSet).forEach(b -> b.run());
    return 0;
  }
}
