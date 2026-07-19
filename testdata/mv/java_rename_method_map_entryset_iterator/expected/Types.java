package demo;

import java.util.Map;

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
  // entrySet().iterator().next().getValue() — Entry of V under foreign same-leaf.
  public static int useIterNextGetValue(Map<String, A> as, Map<String, B> bs) {
    return as.entrySet().iterator().next().getValue().execute()
        + bs.entrySet().iterator().next().getValue().run();
  }

  // var ea = entrySet().iterator().next() then ea.getValue().
  public static int useIterNextVar(Map<String, A> as, Map<String, B> bs) {
    var ea = as.entrySet().iterator().next();
    var eb = bs.entrySet().iterator().next();
    return ea.getValue().execute() + eb.getValue().run();
  }

  // var from getValue on entrySet iterator next.
  public static int useIterNextGetValueVar(Map<String, A> as, Map<String, B> bs) {
    var va = as.entrySet().iterator().next().getValue();
    var vb = bs.entrySet().iterator().next().getValue();
    return va.execute() + vb.run();
  }

  // setValue returns previous V (same leaf as getValue).
  public static int useIterNextSetValue(Map<String, A> as, Map<String, B> bs) {
    return as.entrySet().iterator().next().setValue(new A()).execute()
        + bs.entrySet().iterator().next().setValue(new B()).run();
  }

  // forEachRemaining on entrySet iterator — Entry param, value via getValue.
  public static int useIterForEachRemaining(Map<String, A> as, Map<String, B> bs) {
    as.entrySet().iterator().forEachRemaining(ea -> ea.getValue().execute());
    bs.entrySet().iterator().forEachRemaining(eb -> eb.getValue().run());
    return 0;
  }
}
