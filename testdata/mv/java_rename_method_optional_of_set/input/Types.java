package demo;

import java.util.Optional;
import java.util.Set;
import java.util.List;
import java.util.Collections;

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
  public static int useOptOfSetIterator() {
    return Optional.of(Set.of(new A())).get().iterator().next().run()
        + Optional.of(Set.of(new B())).get().iterator().next().run();
  }

  public static int useOptOfNullableSetIterator() {
    return Optional.ofNullable(Set.of(new A())).get().iterator().next().run()
        + Optional.ofNullable(Set.of(new B())).get().iterator().next().run();
  }

  public static int useOptOfSetOrElseThrow() {
    return Optional.of(Set.of(new A())).orElseThrow().iterator().next().run()
        + Optional.of(Set.of(new B())).orElseThrow().iterator().next().run();
  }

  public static int useOptOfSetStream() {
    return Optional.of(Set.of(new A())).get().stream().mapToInt(xa -> xa.run()).sum()
        + Optional.of(Set.of(new B())).get().stream().mapToInt(xb -> xb.run()).sum();
  }

  public static int useOptOfSetForEach() {
    int[] n = {0};
    Optional.of(Set.of(new A())).get().forEach(a -> n[0] += a.run());
    Optional.of(Set.of(new B())).get().forEach(b -> n[0] += b.run());
    return n[0];
  }

  public static int useOptOfCollectionsSingleton() {
    return Optional.of(Collections.singleton(new A())).get().iterator().next().run()
        + Optional.of(Collections.singleton(new B())).get().iterator().next().run();
  }

  public static int useOptOfListStillWorks() {
    return Optional.of(List.of(new A())).get().get(0).run()
        + Optional.of(List.of(new B())).get().get(0).run();
  }

  public static int useOptOfListOrElseThrow() {
    return Optional.of(List.of(new A())).orElseThrow().get(0).run()
        + Optional.of(List.of(new B())).orElseThrow().get(0).run();
  }

  public static int usePreservesB() {
    return Optional.of(Set.of(new B())).get().iterator().next().run();
  }
}
