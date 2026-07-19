package demo;

import java.util.HashMap;
import java.util.Map;

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
  public static int useForEach(Map<String, A> as, Map<String, B> bs) {
    as.forEach((k, va) -> va.run());
    bs.forEach((k, vb) -> vb.run());
    return 0;
  }

  public static int useValues(Map<String, A> as, Map<String, B> bs) {
    as.values().forEach(va -> va.run());
    bs.values().forEach(vb -> vb.run());
    return 0;
  }

  public static int useHashMap(HashMap<String, A> as, HashMap<String, B> bs) {
    as.forEach((k, va) -> va.run());
    bs.forEach((k, vb) -> vb.run());
    return 0;
  }

  public static int useTypedStill(Map<String, A> as) {
    as.forEach((String k, A val) -> val.run());
    return 0;
  }
}
