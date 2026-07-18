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
  public static int useWindowOptIntermediate() {
    var sa = Stream.of(new A()).gather(Gatherers.windowFixed(1));
    var sb = Stream.of(new B()).gather(Gatherers.windowFixed(1));
    var oa = sa.findFirst();
    var ob = sb.findFirst();
    return oa.get().get(0).execute() + ob.get().get(0).run();
  }

  public static int useWindowOptThenListLocal() {
    var sa = Stream.of(new A()).gather(Gatherers.windowFixed(1));
    var sb = Stream.of(new B()).gather(Gatherers.windowFixed(1));
    var oa = sa.findFirst();
    var ob = sb.findFirst();
    var wa = oa.get();
    var wb = ob.get();
    return wa.get(0).execute() + wb.get(0).run();
  }

  public static int useWindowOptFindAny() {
    var sa = Stream.of(new A()).gather(Gatherers.windowFixed(1));
    var sb = Stream.of(new B()).gather(Gatherers.windowFixed(1));
    var oa = sa.findAny();
    var ob = sb.findAny();
    return oa.get().get(0).execute() + ob.get().get(0).run();
  }

  public static int useWindowOptInlineFindFirst() {
    var oa = Stream.of(new A()).gather(Gatherers.windowFixed(1)).findFirst();
    var ob = Stream.of(new B()).gather(Gatherers.windowFixed(1)).findFirst();
    return oa.get().get(0).execute() + ob.get().get(0).run();
  }

  public static int useWindowOptSliding() {
    var sa = Stream.of(new A()).gather(Gatherers.windowSliding(1));
    var sb = Stream.of(new B()).gather(Gatherers.windowSliding(1));
    var oa = sa.findFirst();
    var ob = sb.findFirst();
    return oa.get().get(0).execute() + ob.get().get(0).run();
  }

  public static int useWindowOptOrElseThrow() {
    var sa = Stream.of(new A()).gather(Gatherers.windowFixed(1));
    var sb = Stream.of(new B()).gather(Gatherers.windowFixed(1));
    var oa = sa.findFirst();
    var ob = sb.findFirst();
    return oa.orElseThrow().get(0).execute() + ob.orElseThrow().get(0).run();
  }

  public static int useFqWindowOpt() {
    var sa = Stream.of(new A()).gather(java.util.stream.Gatherers.windowFixed(1));
    var sb = Stream.of(new B()).gather(java.util.stream.Gatherers.windowFixed(1));
    var oa = sa.findFirst();
    var ob = sb.findFirst();
    return oa.get().get(0).execute() + ob.get().get(0).run();
  }

  public static int usePreservesB() {
    var s = Stream.of(new B()).gather(Gatherers.windowFixed(1));
    var o = s.findFirst();
    return o.get().get(0).run();
  }
}
