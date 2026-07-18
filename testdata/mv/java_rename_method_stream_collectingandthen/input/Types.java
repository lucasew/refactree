package demo;

import java.util.Collections;
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
  public static int useCollectingAndThenForEach(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.collectingAndThen(Collectors.toList(), xs -> xs)).forEach(a -> a.run());
    bs.stream().collect(Collectors.collectingAndThen(Collectors.toList(), xs -> xs)).forEach(b -> b.run());
    return 0;
  }

  public static int useUnmodifiable(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.collectingAndThen(Collectors.toList(), Collections::unmodifiableList)).forEach(a -> a.run());
    bs.stream().collect(Collectors.collectingAndThen(Collectors.toList(), Collections::unmodifiableList)).forEach(b -> b.run());
    return 0;
  }

  public static int useCollectingAndThenToSet(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.collectingAndThen(Collectors.toSet(), xs -> xs)).forEach(a -> a.run());
    bs.stream().collect(Collectors.collectingAndThen(Collectors.toSet(), xs -> xs)).forEach(b -> b.run());
    return 0;
  }

  public static int useVar(List<A> as, List<B> bs) {
    var al = as.stream().collect(Collectors.collectingAndThen(Collectors.toList(), xs -> xs));
    var bl = bs.stream().collect(Collectors.collectingAndThen(Collectors.toList(), xs -> xs));
    al.forEach(a -> a.run());
    bl.forEach(b -> b.run());
    int n = 0;
    for (var a : al) {
      n += a.run();
    }
    for (var b : bl) {
      n += b.run();
    }
    return n;
  }
}
