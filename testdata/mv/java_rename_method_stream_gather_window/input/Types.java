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
  public static int useWindowFixedGet0() {
    return Stream.of(new A())
            .gather(Gatherers.windowFixed(1))
            .findFirst()
            .get()
            .get(0)
            .run()
        + Stream.of(new B())
            .gather(Gatherers.windowFixed(1))
            .findFirst()
            .get()
            .get(0)
            .run();
  }

  public static int useWindowSlidingGet0() {
    return Stream.of(new A())
            .gather(Gatherers.windowSliding(1))
            .findFirst()
            .get()
            .get(0)
            .run()
        + Stream.of(new B())
            .gather(Gatherers.windowSliding(1))
            .findFirst()
            .get()
            .get(0)
            .run();
  }

  public static int useFqWindowFixed() {
    return Stream.of(new A())
            .gather(java.util.stream.Gatherers.windowFixed(1))
            .findFirst()
            .get()
            .get(0)
            .run()
        + Stream.of(new B())
            .gather(java.util.stream.Gatherers.windowFixed(1))
            .findFirst()
            .get()
            .get(0)
            .run();
  }

  public static int useVarWindowList() {
    var wa = Stream.of(new A()).gather(Gatherers.windowFixed(1)).findFirst().get();
    var wb = Stream.of(new B()).gather(Gatherers.windowFixed(1)).findFirst().get();
    return wa.get(0).run() + wb.get(0).run();
  }

  public static int useForEachWindow() {
    Stream.of(new A()).gather(Gatherers.windowFixed(1)).forEach(wa -> wa.get(0).run());
    Stream.of(new B()).gather(Gatherers.windowFixed(1)).forEach(wb -> wb.get(0).run());
    return 0;
  }

  public static int useOrElseThrow() {
    return Stream.of(new A())
            .gather(Gatherers.windowFixed(1))
            .findFirst()
            .orElseThrow()
            .get(0)
            .run()
        + Stream.of(new B())
            .gather(Gatherers.windowFixed(1))
            .findFirst()
            .orElseThrow()
            .get(0)
            .run();
  }

  public static int usePreservesB() {
    return Stream.of(new B())
        .gather(Gatherers.windowFixed(1))
        .findFirst()
        .get()
        .get(0)
        .run();
  }
}
