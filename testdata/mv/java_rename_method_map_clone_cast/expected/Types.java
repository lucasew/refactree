package demo;

import java.util.HashMap;

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
  public static int useCloneCast(HashMap<String, A> as, HashMap<String, B> bs) {
    return ((HashMap<String, A>) as.clone()).get("k").execute()
        + ((HashMap<String, B>) bs.clone()).get("k").run();
  }

  public static int useCloneForEach(HashMap<String, A> as, HashMap<String, B> bs) {
    ((HashMap<String, A>) as.clone()).forEach((k, va) -> va.execute());
    ((HashMap<String, B>) bs.clone()).forEach((k, vb) -> vb.run());
    return 0;
  }

  public static int useCloneVar(HashMap<String, A> as, HashMap<String, B> bs) {
    var am = (HashMap<String, A>) as.clone();
    var bm = (HashMap<String, B>) bs.clone();
    return am.get("k").execute() + bm.get("k").run();
  }

  public static int usePreservesB(HashMap<String, B> bs) {
    var xb = ((HashMap<String, B>) bs.clone()).get("k");
    return ((HashMap<String, B>) bs.clone()).get("k").run() + xb.run();
  }
}
