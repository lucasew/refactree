package demo;

import java.util.stream.Gatherers;
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
  public static int useWindowStreamVarGet0() {
    var sa = Stream.of(new A()).gather(Gatherers.windowFixed(1));
    var sb = Stream.of(new B()).gather(Gatherers.windowFixed(1));
    return sa.findFirst().get().get(0).execute() + sb.findFirst().get().get(0).run();
  }

  public static int useWindowStreamVarFindAny() {
    var sa = Stream.of(new A()).gather(Gatherers.windowFixed(1));
    var sb = Stream.of(new B()).gather(Gatherers.windowFixed(1));
    return sa.findAny().get().get(0).execute() + sb.findAny().get().get(0).run();
  }

  public static int useWindowStreamVarSliding() {
    var sa = Stream.of(new A()).gather(Gatherers.windowSliding(1));
    var sb = Stream.of(new B()).gather(Gatherers.windowSliding(1));
    return sa.findFirst().get().get(0).execute() + sb.findFirst().get().get(0).run();
  }

  public static int useWindowStreamVarListLocal() {
    var sa = Stream.of(new A()).gather(Gatherers.windowFixed(1));
    var sb = Stream.of(new B()).gather(Gatherers.windowFixed(1));
    var wa = sa.findFirst().get();
    var wb = sb.findFirst().get();
    return wa.get(0).execute() + wb.get(0).run();
  }

  public static int useWindowStreamVarForEach() {
    var sa = Stream.of(new A()).gather(Gatherers.windowFixed(1));
    var sb = Stream.of(new B()).gather(Gatherers.windowFixed(1));
    sa.forEach(wa -> wa.get(0).execute());
    sb.forEach(wb -> wb.get(0).run());
    return 0;
  }

  public static int useWindowStreamVarOrElseThrow() {
    var sa = Stream.of(new A()).gather(Gatherers.windowFixed(1));
    var sb = Stream.of(new B()).gather(Gatherers.windowFixed(1));
    return sa.findFirst().orElseThrow().get(0).execute()
        + sb.findFirst().orElseThrow().get(0).run();
  }

  public static int useFqWindowStreamVar() {
    var sa = Stream.of(new A()).gather(java.util.stream.Gatherers.windowFixed(1));
    var sb = Stream.of(new B()).gather(java.util.stream.Gatherers.windowFixed(1));
    return sa.findFirst().get().get(0).execute() + sb.findFirst().get().get(0).run();
  }

  public static int usePreservesB() {
    var s = Stream.of(new B()).gather(Gatherers.windowFixed(1));
    return s.findFirst().get().get(0).run();
  }
}
