package demo;

import java.util.stream.Gatherers;
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
  public static int useFoldNew() {
    return Stream.of("x").gather(Gatherers.fold(() -> new A(), (a, s) -> a)).findFirst().get().run()
        + Stream.of("y").gather(Gatherers.fold(() -> new B(), (b, s) -> b)).findFirst().get().run();
  }

  public static int useScanNew() {
    return Stream.of("x").gather(Gatherers.scan(() -> new A(), (a, s) -> a)).findFirst().get().run()
        + Stream.of("y").gather(Gatherers.scan(() -> new B(), (b, s) -> b)).findFirst().get().run();
  }

  public static int useFqFold() {
    return Stream.of("x")
            .gather(java.util.stream.Gatherers.fold(() -> new A(), (a, s) -> a))
            .findFirst()
            .get()
            .run()
        + Stream.of("y")
            .gather(java.util.stream.Gatherers.fold(() -> new B(), (b, s) -> b))
            .findFirst()
            .get()
            .run();
  }

  public static int useVarFold() {
    var sa = Stream.of("x").gather(Gatherers.fold(() -> new A(), (a, s) -> a));
    var sb = Stream.of("y").gather(Gatherers.fold(() -> new B(), (b, s) -> b));
    return sa.findFirst().get().run() + sb.findFirst().get().run();
  }

  public static int usePreservesB() {
    return Stream.of("y").gather(Gatherers.fold(() -> new B(), (b, s) -> b)).findFirst().get().run();
  }
}
