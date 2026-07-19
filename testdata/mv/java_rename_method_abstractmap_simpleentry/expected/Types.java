package demo;

import java.util.AbstractMap;
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
  // new AbstractMap.SimpleEntry<>(k, new A()).getValue() — V under foreign same-leaf.
  public static int useSimpleEntryInline() {
    return new AbstractMap.SimpleEntry<>("k", new A()).getValue().execute()
        + new AbstractMap.SimpleEntry<>("k", new B()).getValue().run();
  }

  // var from SimpleEntry creation.
  public static int useSimpleEntryVar() {
    var ea = new AbstractMap.SimpleEntry<>("k", new A());
    var eb = new AbstractMap.SimpleEntry<>("k", new B());
    return ea.getValue().execute() + eb.getValue().run();
  }

  // var from getValue on SimpleEntry.
  public static int useSimpleEntryGetValueVar() {
    var va = new AbstractMap.SimpleEntry<>("k", new A()).getValue();
    var vb = new AbstractMap.SimpleEntry<>("k", new B()).getValue();
    return va.execute() + vb.run();
  }

  // SimpleImmutableEntry same V leaf.
  public static int useSimpleImmutableEntry() {
    return new AbstractMap.SimpleImmutableEntry<>("k", new A()).getValue().execute()
        + new AbstractMap.SimpleImmutableEntry<>("k", new B()).getValue().run();
  }

  // Typed SimpleEntry local (explicit type args).
  public static int useTypedSimpleEntry() {
    AbstractMap.SimpleEntry<String, A> ea =
        new AbstractMap.SimpleEntry<>("k", new A());
    AbstractMap.SimpleEntry<String, B> eb =
        new AbstractMap.SimpleEntry<>("k", new B());
    return ea.getValue().execute() + eb.getValue().run();
  }

  // Regression: Map.entry already worked.
  public static int useMapEntry() {
    return Map.entry("k", new A()).getValue().execute()
        + Map.entry("k", new B()).getValue().run();
  }
}
