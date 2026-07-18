package demo;

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
  public static int useFQNCopies() {
    java.util.Collections.nCopies(1, new A()).forEach(x1 -> x1.execute());
    java.util.Collections.nCopies(1, new B()).forEach(y1 -> y1.run());
    return 0;
  }

  public static int useFQSingletonList() {
    java.util.Collections.singletonList(new A()).forEach(x2 -> x2.execute());
    java.util.Collections.singletonList(new B()).forEach(y2 -> y2.run());
    return 0;
  }

  public static int useFQUnmod(java.util.List<A> as, java.util.List<B> bs) {
    java.util.Collections.unmodifiableList(as).forEach(x3 -> x3.execute());
    java.util.Collections.unmodifiableList(bs).forEach(y3 -> y3.run());
    return 0;
  }

  public static int useFQMapOf() {
    return java.util.Map.of("k", new A()).get("k").execute()
        + java.util.Map.of("k", new B()).get("k").run();
  }

  public static int useFQSingletonMap() {
    return java.util.Collections.singletonMap("k", new A()).get("k").execute()
        + java.util.Collections.singletonMap("k", new B()).get("k").run();
  }

  public static int useFQOfEntries() {
    return java.util.Map.ofEntries(java.util.Map.entry("k", new A())).get("k").execute()
        + java.util.Map.ofEntries(java.util.Map.entry("k", new B())).get("k").run();
  }

  public static int useFQAsList() {
    java.util.Arrays.asList(new A()).forEach(x4 -> x4.execute());
    java.util.Arrays.asList(new B()).forEach(y4 -> y4.run());
    return 0;
  }

  public static int usePreservesB() {
    return java.util.Map.of("k", new B()).get("k").run()
        + java.util.Collections.nCopies(1, new B()).stream().findFirst().get().run();
  }
}
