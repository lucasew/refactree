package demo;

import java.util.ArrayList;
import java.util.HashSet;
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
  public static int useToCollectionForEach(List<A> as, List<B> bs) {
    as.stream().collect(Collectors.toCollection(ArrayList::new)).forEach(a -> a.execute());
    bs.stream().collect(Collectors.toCollection(ArrayList::new)).forEach(b -> b.run());
    return 0;
  }

  public static int useToCollectionFor(List<A> as, List<B> bs) {
    int n = 0;
    for (var a : as.stream().collect(Collectors.toCollection(ArrayList::new))) {
      n += a.execute();
    }
    for (var b : bs.stream().collect(Collectors.toCollection(HashSet::new))) {
      n += b.run();
    }
    return n;
  }

  public static int useVarToCollection(List<A> as, List<B> bs) {
    var al = as.stream().collect(Collectors.toCollection(ArrayList::new));
    var bl = bs.stream().collect(Collectors.toCollection(HashSet::new));
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
