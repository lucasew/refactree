package demo;

import java.util.AbstractMap;
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
  // new AbstractMap.SimpleEntry<>(k, new A()).getValue() — V under foreign same-leaf.
  public static int useSimpleEntryInline() {
    return new AbstractMap.SimpleEntry<>("k", new A()).getValue().run()
        + new AbstractMap.SimpleEntry<>("k", new B()).getValue().run();
  }

  // var from SimpleEntry creation.
  public static int useSimpleEntryVar() {
    var ea = new AbstractMap.SimpleEntry<>("k", new A());
    var eb = new AbstractMap.SimpleEntry<>("k", new B());
    return ea.getValue().run() + eb.getValue().run();
  }

  // var from getValue on SimpleEntry.
  public static int useSimpleEntryGetValueVar() {
    var va = new AbstractMap.SimpleEntry<>("k", new A()).getValue();
    var vb = new AbstractMap.SimpleEntry<>("k", new B()).getValue();
    return va.run() + vb.run();
  }

  // SimpleImmutableEntry same V leaf.
  public static int useSimpleImmutableEntry() {
    return new AbstractMap.SimpleImmutableEntry<>("k", new A()).getValue().run()
        + new AbstractMap.SimpleImmutableEntry<>("k", new B()).getValue().run();
  }

  // Typed SimpleEntry local (explicit type args).
  public static int useTypedSimpleEntry() {
    AbstractMap.SimpleEntry<String, A> ea =
        new AbstractMap.SimpleEntry<>("k", new A());
    AbstractMap.SimpleEntry<String, B> eb =
        new AbstractMap.SimpleEntry<>("k", new B());
    return ea.getValue().run() + eb.getValue().run();
  }

  // Regression: Map.entry already worked.
  public static int useMapEntry() {
    return Map.entry("k", new A()).getValue().run()
        + Map.entry("k", new B()).getValue().run();
  }
}
