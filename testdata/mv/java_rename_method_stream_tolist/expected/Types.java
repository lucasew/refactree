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
  public static int useToListForEach(List<A> as, List<B> bs) {
    as.stream().toList().forEach(a -> a.execute());
    bs.stream().toList().forEach(b -> b.run());
    return 0;
  }

  public static int useToListFor(List<A> as, List<B> bs) {
    int n = 0;
    for (var a : as.stream().toList()) {
      n += a.execute();
    }
    for (var b : bs.stream().toList()) {
      n += b.run();
    }
    return n;
  }

  public static int useCollectToList(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.toList()).forEach(a -> a.execute());
    bs.stream().collect(Collectors.toList()).forEach(b -> b.run());
    return 0;
  }

  public static int useCollectMethodRef(List<A> as, List<B> bs) {
    as.stream().collect(Collectors::toList).forEach(a -> a.execute());
    bs.stream().collect(Collectors::toList).forEach(b -> b.run());
    return 0;
  }

  public static int useVarToList(List<A> as, List<B> bs) {
    var al = as.stream().toList();
    var bl = bs.stream().toList();
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

  public static int useVarCollect(List<A> as, List<B> bs) {
    var al = as.stream().collect(Collectors.toList());
    var bl = bs.stream().collect(Collectors.toList());
    al.forEach(a -> a.execute());
    bl.forEach(b -> b.run());
    return 0;
  }
}
