package demo;

import java.util.Arrays;
import java.util.Collections;
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
  public static int useListOfGet() {
    var xa = List.of(new A()).get(0);
    var xb = List.of(new B()).get(0);
    return xa.execute() + xb.run();
  }

  public static int useListOfGetFirstLast() {
    var xa = List.of(new A()).getFirst();
    var xb = List.of(new B()).getLast();
    return xa.execute() + xb.run();
  }

  public static int useArraysAsListGet() {
    var xa = Arrays.asList(new A()).get(0);
    var xb = Arrays.asList(new B()).get(0);
    return xa.execute() + xb.run();
  }

  public static int useSingletonListGet() {
    var xa = Collections.singletonList(new A()).get(0);
    var xb = Collections.singletonList(new B()).get(0);
    return xa.execute() + xb.run();
  }

  public static int useStreamToListGet(List<A> as, List<B> bs) {
    var xa = as.stream().toList().get(0);
    var xb = bs.stream().toList().get(0);
    return xa.execute() + xb.run();
  }

  public static int useListOfMultiGet() {
    var xa = List.of(new A(), new A()).get(1);
    var xb = List.of(new B(), new B()).get(1);
    return xa.execute() + xb.run();
  }

  public static int useListCopyOfGet(List<A> as, List<B> bs) {
    var xa = List.copyOf(as).get(0);
    var xb = List.copyOf(bs).get(0);
    return xa.execute() + xb.run();
  }
}
