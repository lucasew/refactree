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
  public static int useToUnmodifiableListForEach(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.toUnmodifiableList()).forEach(a -> a.run());
    bs.stream().collect(Collectors.toUnmodifiableList()).forEach(b -> b.run());
    return 0;
  }

  public static int useToUnmodifiableSetForEach(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.toUnmodifiableSet()).forEach(a -> a.run());
    bs.stream().collect(Collectors.toUnmodifiableSet()).forEach(b -> b.run());
    return 0;
  }

  public static int useToUnmodifiableListFor(List<A> as, List<B> bs) {
    int n = 0;
    for (var a : as.stream().collect(Collectors.toUnmodifiableList())) {
      n += a.run();
    }
    for (var b : bs.stream().collect(Collectors.toUnmodifiableList())) {
      n += b.run();
    }
    return n;
  }

  public static int useVarToUnmodifiableList(List<A> as, List<B> bs) {
    var al = as.stream().collect(Collectors.toUnmodifiableList());
    var bl = bs.stream().collect(Collectors.toUnmodifiableList());
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

  public static int useVarToUnmodifiableSet(List<A> as, List<B> bs) {
    var al = as.stream().collect(Collectors.toUnmodifiableSet());
    var bl = bs.stream().collect(Collectors.toUnmodifiableSet());
    al.forEach(a -> a.run());
    bl.forEach(b -> b.run());
    return 0;
  }

  public static int useMethodRefList(List<A> as, List<B> bs) {
    as.stream().collect(Collectors::toUnmodifiableList).forEach(a -> a.run());
    bs.stream().collect(Collectors::toUnmodifiableList).forEach(b -> b.run());
    return 0;
  }

  public static int useMethodRefSet(List<A> as, List<B> bs) {
    as.stream().collect(Collectors::toUnmodifiableSet).forEach(a -> a.run());
    bs.stream().collect(Collectors::toUnmodifiableSet).forEach(b -> b.run());
    return 0;
  }
}
