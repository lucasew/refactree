package demo;

import java.util.Optional;
import java.util.Set;
import java.util.List;

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
  public static int useOptOfSetCopyOfListOfIterator() {
    return Optional.of(Set.copyOf(List.of(new A()))).get().iterator().next().run()
        + Optional.of(Set.copyOf(List.of(new B()))).get().iterator().next().run();
  }

  public static int useOptOfNullableSetCopyOfListOf() {
    return Optional.ofNullable(Set.copyOf(List.of(new A()))).get().iterator().next().run()
        + Optional.ofNullable(Set.copyOf(List.of(new B()))).get().iterator().next().run();
  }

  public static int useOptOfSetCopyOfListOfOrElseThrow() {
    return Optional.of(Set.copyOf(List.of(new A()))).orElseThrow().iterator().next().run()
        + Optional.of(Set.copyOf(List.of(new B()))).orElseThrow().iterator().next().run();
  }

  public static int useOptOfSetCopyOfListOfStream() {
    return Optional.of(Set.copyOf(List.of(new A()))).get().stream().mapToInt(xa -> xa.run()).sum()
        + Optional.of(Set.copyOf(List.of(new B()))).get().stream().mapToInt(xb -> xb.run()).sum();
  }

  public static int useOptOfSetCopyOfListOfForEach() {
    int[] n = {0};
    Optional.of(Set.copyOf(List.of(new A()))).get().forEach(a -> n[0] += a.run());
    Optional.of(Set.copyOf(List.of(new B()))).get().forEach(b -> n[0] += b.run());
    return n[0];
  }

  public static int useOptOfListCopyOfListOfGet() {
    return Optional.of(List.copyOf(List.of(new A()))).get().get(0).run()
        + Optional.of(List.copyOf(List.of(new B()))).get().get(0).run();
  }

  public static int useOptOfListCopyOfListOfOrElseThrow() {
    return Optional.of(List.copyOf(List.of(new A()))).orElseThrow().get(0).run()
        + Optional.of(List.copyOf(List.of(new B()))).orElseThrow().get(0).run();
  }

  public static int useOptOfSetOfStillWorks() {
    return Optional.of(Set.of(new A())).get().iterator().next().run()
        + Optional.of(Set.of(new B())).get().iterator().next().run();
  }

  public static int usePreservesB() {
    return Optional.of(Set.copyOf(List.of(new B()))).get().iterator().next().run();
  }
}
