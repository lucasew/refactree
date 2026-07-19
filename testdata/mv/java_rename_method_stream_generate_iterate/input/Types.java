package demo;

import java.util.stream.Stream;

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
  public static int useOfNullableForEach() {
    Stream.ofNullable(new A()).forEach(a -> a.run());
    Stream.ofNullable(new B()).forEach(b -> b.run());
    return 0;
  }

  public static int useGenerateForEach() {
    Stream.generate(() -> new A()).limit(1).forEach(a -> a.run());
    Stream.generate(() -> new B()).limit(1).forEach(b -> b.run());
    return 0;
  }

  public static int useIterateForEach() {
    Stream.iterate(new A(), a -> a).limit(1).forEach(a -> a.run());
    Stream.iterate(new B(), b -> b).limit(1).forEach(b -> b.run());
    return 0;
  }

  public static int useIteratePredForEach() {
    Stream.iterate(new A(), a -> true, a -> a).limit(1).forEach(a -> a.run());
    Stream.iterate(new B(), b -> true, b -> b).limit(1).forEach(b -> b.run());
    return 0;
  }

  public static int useOfNullableToListFor() {
    int n = 0;
    for (var a : Stream.ofNullable(new A()).toList()) {
      n += a.run();
    }
    for (var b : Stream.ofNullable(new B()).toList()) {
      n += b.run();
    }
    return n;
  }

  public static int useVarGenerate() {
    var sa = Stream.generate(() -> new A()).limit(1);
    var sb = Stream.generate(() -> new B()).limit(1);
    sa.forEach(a -> a.run());
    sb.forEach(b -> b.run());
    int n = 0;
    for (var a : sa.toList()) {
      n += a.run();
    }
    for (var b : sb.toList()) {
      n += b.run();
    }
    return n;
  }

  public static int useVarIterate() {
    var sa = Stream.iterate(new A(), a -> a).limit(1);
    var sb = Stream.iterate(new B(), b -> b).limit(1);
    sa.forEach(a -> a.run());
    sb.forEach(b -> b.run());
    return 0;
  }
}
