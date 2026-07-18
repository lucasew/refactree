package demo;

import java.util.List;
import java.util.Optional;
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
  public static int useOptListGet(Optional<List<A>> oa, Optional<List<B>> ob) {
    return oa.get().get(0).execute() + ob.get().get(0).run();
  }

  public static int useOptListVar(Optional<List<A>> oa, Optional<List<B>> ob) {
    var ga = oa.get();
    var gb = ob.get();
    return ga.get(0).execute() + gb.get(0).run();
  }

  public static int useOptListOrElseThrow(Optional<List<A>> oa, Optional<List<B>> ob) {
    return oa.orElseThrow().get(0).execute() + ob.orElseThrow().get(0).run();
  }

  public static int useSetOfList(Set<List<A>> sa, Set<List<B>> sb) {
    int n = 0;
    for (var ga : sa) {
      n += ga.get(0).execute();
    }
    for (var gb : sb) {
      n += gb.get(0).run();
    }
    return n;
  }

  public static int usePreservesB(Optional<List<B>> ob) {
    return ob.get().get(0).run();
  }
}
