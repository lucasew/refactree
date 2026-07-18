package demo;

import java.util.Arrays;
import java.util.Collections;
import java.util.List;
import java.util.Optional;

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
  public static int useOptOfListGet() {
    return Optional.of(List.of(new A())).get().get(0).execute()
        + Optional.of(List.of(new B())).get().get(0).run();
  }

  public static int useOptOfNullableListGet() {
    return Optional.ofNullable(List.of(new A())).get().get(0).execute()
        + Optional.ofNullable(List.of(new B())).get().get(0).run();
  }

  public static int useOptOfArraysAsList() {
    return Optional.of(Arrays.asList(new A())).get().get(0).execute()
        + Optional.of(Arrays.asList(new B())).get().get(0).run();
  }

  public static int useOptOfSingletonList() {
    return Optional.of(Collections.singletonList(new A())).get().get(0).execute()
        + Optional.of(Collections.singletonList(new B())).get().get(0).run();
  }

  public static int useOptOfListOrElseThrow() {
    return Optional.of(List.of(new A())).orElseThrow().get(0).execute()
        + Optional.of(List.of(new B())).orElseThrow().get(0).run();
  }

  public static int usePreservesB() {
    return Optional.of(List.of(new B())).get().get(0).run();
  }
}
