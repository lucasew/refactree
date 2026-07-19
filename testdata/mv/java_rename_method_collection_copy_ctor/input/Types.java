package demo;

import java.util.ArrayList;
import java.util.HashSet;
import java.util.LinkedList;
import java.util.List;

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
  public static int useArrayListOfGet() {
    return new ArrayList<>(List.of(new A())).get(0).run()
        + new ArrayList<>(List.of(new B())).get(0).run();
  }

  public static int useLinkedListLocal(List<A> as, List<B> bs) {
    return new LinkedList<>(as).getFirst().run()
        + new LinkedList<>(bs).getFirst().run();
  }

  public static int useHashSetOfForEach() {
    new HashSet<>(List.of(new A())).forEach(a -> a.run());
    new HashSet<>(List.of(new B())).forEach(b -> b.run());
    return 0;
  }

  public static int useVarCopy(List<A> as, List<B> bs) {
    var al = new ArrayList<>(as);
    var bl = new ArrayList<>(bs);
    al.forEach(a -> a.run());
    bl.forEach(b -> b.run());
    return 0;
  }

  public static int usePreservesB(List<B> bs) {
    return new ArrayList<>(bs).get(0).run()
        + new LinkedList<>(List.of(new B())).getFirst().run();
  }
}
