package demo;

import java.util.Arrays;
import java.util.Collections;
import java.util.List;
import java.util.Optional;
import java.util.Set;

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

class BoxA {
  private final A a = new A();

  public A get() {
    return a;
  }
}

class BoxB {
  private final B b = new B();

  public B get() {
    return b;
  }
}

class Uses {
  // Class regressions — already solid.
  public static int useClassOptOfListGet() {
    return Optional.of(List.of(new A())).get().get(0).execute()
        + Optional.of(List.of(new B())).get().get(0).run();
  }

  public static int useClassOptOfNullableListGet() {
    return Optional.ofNullable(List.of(new A())).get().get(0).execute()
        + Optional.ofNullable(List.of(new B())).get().get(0).run();
  }

  public static int useClassOptOfArraysAsList() {
    return Optional.of(Arrays.asList(new A())).get().get(0).execute()
        + Optional.of(Arrays.asList(new B())).get().get(0).run();
  }

  public static int useClassOptOfSingletonList() {
    return Optional.of(Collections.singletonList(new A())).get().get(0).execute()
        + Optional.of(Collections.singletonList(new B())).get().get(0).run();
  }

  public static int useClassOptOfListOrElseThrow() {
    return Optional.of(List.of(new A())).orElseThrow().get(0).execute()
        + Optional.of(List.of(new B())).orElseThrow().get(0).run();
  }

  public static int useClassOptOfSetIterator() {
    return Optional.of(Set.of(new A())).get().iterator().next().execute()
        + Optional.of(Set.of(new B())).get().iterator().next().run();
  }

  // Method-return under foreign same-leaf.
  public static int useMrOptOfListGet(BoxA ba, BoxB bb) {
    return Optional.of(List.of(ba.get())).get().get(0).execute()
        + Optional.of(List.of(bb.get())).get().get(0).run();
  }

  public static int useMrOptOfNullableListGet(BoxA ba, BoxB bb) {
    return Optional.ofNullable(List.of(ba.get())).get().get(0).execute()
        + Optional.ofNullable(List.of(bb.get())).get().get(0).run();
  }

  public static int useMrOptOfArraysAsList(BoxA ba, BoxB bb) {
    return Optional.of(Arrays.asList(ba.get())).get().get(0).execute()
        + Optional.of(Arrays.asList(bb.get())).get().get(0).run();
  }

  public static int useMrOptOfSingletonList(BoxA ba, BoxB bb) {
    return Optional.of(Collections.singletonList(ba.get())).get().get(0).execute()
        + Optional.of(Collections.singletonList(bb.get())).get().get(0).run();
  }

  public static int useMrOptOfListOrElseThrow(BoxA ba, BoxB bb) {
    return Optional.of(List.of(ba.get())).orElseThrow().get(0).execute()
        + Optional.of(List.of(bb.get())).orElseThrow().get(0).run();
  }

  public static int useMrOptOfSetIterator(BoxA ba, BoxB bb) {
    return Optional.of(Set.of(ba.get())).get().iterator().next().execute()
        + Optional.of(Set.of(bb.get())).get().iterator().next().run();
  }

  public static int useMrOptOfListAssign(BoxA ba, BoxB bb) {
    var ga = Optional.of(List.of(ba.get())).get();
    var gb = Optional.of(List.of(bb.get())).get();
    return ga.get(0).execute() + gb.get(0).run();
  }

  public static int useMrOptOfListNewBox() {
    return Optional.of(List.of(new BoxA().get())).get().get(0).execute()
        + Optional.of(List.of(new BoxB().get())).get().get(0).run();
  }

  public static int usePreservesB(BoxB bb) {
    return Optional.of(List.of(bb.get())).get().get(0).run()
        + Optional.ofNullable(List.of(bb.get())).get().get(0).run()
        + Optional.of(Arrays.asList(bb.get())).get().get(0).run()
        + Optional.of(Collections.singletonList(bb.get())).get().get(0).run()
        + Optional.of(List.of(bb.get())).orElseThrow().get(0).run()
        + Optional.of(Set.of(bb.get())).get().iterator().next().run()
        + new B().run();
  }
}
