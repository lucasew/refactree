package demo;

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
  public static int useFQStreamOf() {
    return java.util.stream.Stream.of(new A()).map(a -> a.run()).mapToInt(i -> i).sum()
        + java.util.stream.Stream.of(new B()).map(b -> b.run()).mapToInt(i -> i).sum();
  }

  public static int useFQListOf() {
    java.util.List.of(new A()).forEach(a -> a.run());
    java.util.List.of(new B()).forEach(b -> b.run());
    return 0;
  }

  public static int useFQStreamOfNullable() {
    java.util.stream.Stream.ofNullable(new A()).forEach(a -> a.run());
    java.util.stream.Stream.ofNullable(new B()).forEach(b -> b.run());
    return 0;
  }

  public static int useFQOptional() {
    return java.util.Optional.of(new A()).get().run()
        + java.util.Optional.of(new B()).get().run();
  }

  public static int usePreservesB() {
    return java.util.stream.Stream.of(new B()).map(b -> b.run()).mapToInt(i -> i).sum();
  }
}
