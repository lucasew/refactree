package demo;

import java.util.List;
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
  public static int useMapConcurrentIdentity() {
    return Stream.of(new A())
            .gather(Gatherers.mapConcurrent(1, a -> a))
            .findFirst()
            .get()
            .run()
        + Stream.of(new B())
            .gather(Gatherers.mapConcurrent(1, b -> b))
            .findFirst()
            .get()
            .run();
  }

  public static int useVarPipeline() {
    var sa = Stream.of(new A()).gather(Gatherers.mapConcurrent(1, a -> a));
    var sb = Stream.of(new B()).gather(Gatherers.mapConcurrent(1, b -> b));
    return sa.findFirst().get().run() + sb.findFirst().get().run();
  }

  public static int useForEach(List<A> as, List<B> bs) {
    as.stream().gather(Gatherers.mapConcurrent(1, a -> a)).forEach(x -> x.run());
    bs.stream().gather(Gatherers.mapConcurrent(1, b -> b)).forEach(y -> y.run());
    return 0;
  }

  public static int useMapConcurrentNew() {
    return Stream.of("x")
            .gather(Gatherers.mapConcurrent(1, s -> new A()))
            .findFirst()
            .get()
            .run()
        + Stream.of("y")
            .gather(Gatherers.mapConcurrent(1, s -> new B()))
            .findFirst()
            .get()
            .run();
  }

  public static int usePreservesB() {
    return Stream.of(new B())
        .gather(Gatherers.mapConcurrent(1, b -> b))
        .findFirst()
        .get()
        .run();
  }
}
