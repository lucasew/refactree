package demo;

import java.util.Arrays;
import java.util.List;
import java.util.stream.Stream;

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
  public static int useStreamOf() {
    return Stream.of(new A()).map(a -> a.execute()).mapToInt(i -> i).sum();
  }

  public static int useStreamOfB() {
    return Stream.of(new B()).map(b -> b.run()).mapToInt(i -> i).sum();
  }

  public static int useListOf() {
    return List.of(new A()).stream().map(a -> a.execute()).mapToInt(i -> i).sum();
  }

  public static int useListOfForEach() {
    List.of(new A()).forEach(a -> a.execute());
    List.of(new B()).forEach(b -> b.run());
    return 0;
  }

  public static int useStreamOfMulti() {
    Stream.of(new A(), new A()).forEach(a -> a.execute());
    Stream.of(new B(), new B()).forEach(b -> b.run());
    return 0;
  }

  public static int useArraysAsList() {
    Arrays.asList(new A()).forEach(a -> a.execute());
    Arrays.asList(new B()).forEach(b -> b.run());
    return 0;
  }

  public static int useTypedStill() {
    return Stream.of(new A()).map((A a) -> a.execute()).mapToInt(i -> i).sum();
  }
}
