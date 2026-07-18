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
  public static int useListOfRemoveFirst() {
    var xa = List.of(new A()).removeFirst();
    var xb = List.of(new B()).removeLast();
    return xa.execute() + xb.run();
  }

  public static int useArraysAsListRemove() {
    var xa = Arrays.asList(new A()).removeFirst();
    var xb = Arrays.asList(new B()).removeLast();
    return xa.execute() + xb.run();
  }

  public static int useSingletonListRemove() {
    var xa = Collections.singletonList(new A()).removeFirst();
    var xb = Collections.singletonList(new B()).removeLast();
    return xa.execute() + xb.run();
  }

  public static int useStreamToListRemove(List<A> as, List<B> bs) {
    var xa = as.stream().toList().remove(0);
    var xb = bs.stream().toList().remove(0);
    return xa.execute() + xb.run();
  }

  public static int useStreamToListRemoveFirst(List<A> as, List<B> bs) {
    var xa = as.stream().toList().removeFirst();
    var xb = bs.stream().toList().removeLast();
    return xa.execute() + xb.run();
  }

  public static int useListCopyOfRemove(List<A> as, List<B> bs) {
    var xa = List.copyOf(as).removeFirst();
    var xb = List.copyOf(bs).removeLast();
    return xa.execute() + xb.run();
  }

  public static int useReversedRemove(List<A> as, List<B> bs) {
    var xa = as.reversed().removeFirst();
    var xb = bs.reversed().removeLast();
    return xa.execute() + xb.run();
  }
}
