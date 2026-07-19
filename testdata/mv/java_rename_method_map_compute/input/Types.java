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
  public static int useComputeIfPresent(Map<String, A> as, Map<String, B> bs) {
    as.computeIfPresent("k", (k, va) -> {
      va.run();
      return va;
    });
    bs.computeIfPresent("k", (k, vb) -> {
      vb.run();
      return vb;
    });
    return 0;
  }

  public static int useCompute(Map<String, A> as, Map<String, B> bs) {
    as.compute("k", (k, va) -> {
      if (va != null) {
        va.run();
      }
      return va;
    });
    bs.compute("k", (k, vb) -> {
      if (vb != null) {
        vb.run();
      }
      return vb;
    });
    return 0;
  }

  public static int useReplaceAll(Map<String, A> as, Map<String, B> bs) {
    as.replaceAll((k, va) -> {
      va.run();
      return va;
    });
    bs.replaceAll((k, vb) -> {
      vb.run();
      return vb;
    });
    return 0;
  }

  public static int useMerge(Map<String, A> as, Map<String, B> bs) {
    as.merge("k", new A(), (olda, newa) -> {
      olda.run();
      return olda;
    });
    bs.merge("k", new B(), (oldb, newb) -> {
      oldb.run();
      return oldb;
    });
    return 0;
  }

  public static int useHashMap(HashMap<String, A> as) {
    as.computeIfPresent("k", (k, va) -> {
      va.run();
      return va;
    });
    return 0;
  }

  public static int useTypedStill(Map<String, A> as) {
    as.computeIfPresent("k", (String k, A val) -> {
      val.run();
      return val;
    });
    return 0;
  }
}
