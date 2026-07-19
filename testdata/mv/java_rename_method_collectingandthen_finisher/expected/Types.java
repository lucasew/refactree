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
  public static int useFinisherGet0(List<A> as, List<B> bs) {
    return as.stream().collect(Collectors.collectingAndThen(Collectors.toList(), l -> l.get(0))).execute()
        + bs.stream().collect(Collectors.collectingAndThen(Collectors.toList(), l -> l.get(0))).run();
  }

  public static int useFinisherGet0Assign(List<A> as, List<B> bs) {
    var xa = as.stream().collect(Collectors.collectingAndThen(Collectors.toList(), l -> l.get(0)));
    var xb = bs.stream().collect(Collectors.collectingAndThen(Collectors.toList(), l -> l.get(0)));
    return xa.execute() + xb.run();
  }

  public static int useFinisherToSet(List<A> as, List<B> bs) {
    return as.stream().collect(Collectors.collectingAndThen(Collectors.toSet(), l -> l.iterator().next())).execute()
        + bs.stream().collect(Collectors.collectingAndThen(Collectors.toSet(), l -> l.iterator().next())).run();
  }

  public static int useFinisherGetFirst(List<A> as, List<B> bs) {
    return as.stream().collect(Collectors.collectingAndThen(Collectors.toList(), l -> l.getFirst())).execute()
        + bs.stream().collect(Collectors.collectingAndThen(Collectors.toList(), l -> l.getFirst())).run();
  }

  public static int useFinisherMethodRef(List<A> as, List<B> bs) {
    return as.stream().collect(Collectors.collectingAndThen(Collectors.toList(), List::getFirst)).execute()
        + bs.stream().collect(Collectors.collectingAndThen(Collectors.toList(), List::getFirst)).run();
  }

  public static int useIdentityStillList(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.collectingAndThen(Collectors.toList(), xs -> xs)).forEach(a -> a.execute());
    bs.stream().collect(Collectors.collectingAndThen(Collectors.toList(), xs -> xs)).forEach(b -> b.run());
    return 0;
  }

  public static int usePreservesB(List<B> bs) {
    return bs.stream().collect(Collectors.collectingAndThen(Collectors.toList(), l -> l.get(0))).run();
  }
}
