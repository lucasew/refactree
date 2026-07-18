package demo;

import java.util.LinkedHashMap;
import java.util.SequencedMap;

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
  public static int usePutFirst(SequencedMap<String, A> am, SequencedMap<String, B> bm) {
    var xa = am.putFirst("k", new A());
    var xb = bm.putFirst("k", new B());
    return xa.execute() + xb.run();
  }

  public static int usePutLast(SequencedMap<String, A> am, SequencedMap<String, B> bm) {
    var ya = am.putLast("k", new A());
    var yb = bm.putLast("k", new B());
    return ya.execute() + yb.run();
  }

  public static int useLinkedHashMap(LinkedHashMap<String, A> am, LinkedHashMap<String, B> bm) {
    var za = am.putFirst("k", new A());
    var zb = bm.putFirst("k", new B());
    return za.execute() + zb.run();
  }
}
