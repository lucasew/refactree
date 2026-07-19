package demo;

import java.util.List;
import java.util.Set;

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
  public static int useListCopyOfForEach(List<A> as, List<B> bs) {
    List.copyOf(as).forEach(a -> a.execute());
    List.copyOf(bs).forEach(b -> b.run());
    return 0;
  }

  public static int useSetCopyOfForEach(List<A> as, List<B> bs) {
    Set.copyOf(as).forEach(a -> a.execute());
    Set.copyOf(bs).forEach(b -> b.run());
    return 0;
  }

  public static int useListCopyOfFor(List<A> as, List<B> bs) {
    int n = 0;
    for (var a : List.copyOf(as)) {
      n += a.execute();
    }
    for (var b : List.copyOf(bs)) {
      n += b.run();
    }
    return n;
  }

  public static int useVarListCopyOf(List<A> as, List<B> bs) {
    var al = List.copyOf(as);
    var bl = List.copyOf(bs);
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

  public static int useVarSetCopyOf(List<A> as, List<B> bs) {
    var al = Set.copyOf(as);
    var bl = Set.copyOf(bs);
    al.forEach(a -> a.execute());
    bl.forEach(b -> b.run());
    return 0;
  }
}
