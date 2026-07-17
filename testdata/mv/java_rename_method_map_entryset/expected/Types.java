package demo;

import java.util.HashMap;
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
  public static int useForEach(Map<String, A> as, Map<String, B> bs) {
    as.entrySet().forEach(ea -> ea.getValue().execute());
    bs.entrySet().forEach(eb -> eb.getValue().run());
    return 0;
  }

  public static int useForVar(Map<String, A> as, Map<String, B> bs) {
    int n = 0;
    for (var ea : as.entrySet()) {
      n += ea.getValue().execute();
    }
    for (var eb : bs.entrySet()) {
      n += eb.getValue().run();
    }
    return n;
  }

  public static int useTypedEntry(Map<String, A> as, Map<String, B> bs) {
    int n = 0;
    for (Map.Entry<String, A> ea : as.entrySet()) {
      n += ea.getValue().execute();
    }
    for (Map.Entry<String, B> eb : bs.entrySet()) {
      n += eb.getValue().run();
    }
    return n;
  }

  public static int useGetValueVar(Map<String, A> as, Map<String, B> bs) {
    int n = 0;
    for (var ea : as.entrySet()) {
      var va = ea.getValue();
      n += va.execute();
    }
    for (var eb : bs.entrySet()) {
      var vb = eb.getValue();
      n += vb.run();
    }
    return n;
  }

  public static int useStream(Map<String, A> as, Map<String, B> bs) {
    as.entrySet().stream().forEach(ea -> ea.getValue().execute());
    bs.entrySet().stream().forEach(eb -> eb.getValue().run());
    return 0;
  }

  public static int useHashMap(HashMap<String, A> as, HashMap<String, B> bs) {
    as.entrySet().forEach(ea -> ea.getValue().execute());
    bs.entrySet().forEach(eb -> eb.getValue().run());
    return 0;
  }
}
