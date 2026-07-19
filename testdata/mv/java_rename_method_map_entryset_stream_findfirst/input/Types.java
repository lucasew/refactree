package demo;

import java.util.Map;
import java.util.Optional;

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
  // entrySet().stream().findFirst().get().getValue() — Entry of V under foreign same-leaf.
  public static int useFindFirstGet(Map<String, A> as, Map<String, B> bs) {
    return as.entrySet().stream().findFirst().get().getValue().run()
        + bs.entrySet().stream().findFirst().get().getValue().run();
  }

  // var ea = entrySet().stream().findFirst().get() then ea.getValue().
  public static int useFindFirstGetVar(Map<String, A> as, Map<String, B> bs) {
    var ea = as.entrySet().stream().findFirst().get();
    var eb = bs.entrySet().stream().findFirst().get();
    return ea.getValue().run() + eb.getValue().run();
  }

  // var from getValue on entrySet stream findFirst get.
  public static int useFindFirstGetValueVar(Map<String, A> as, Map<String, B> bs) {
    var va = as.entrySet().stream().findFirst().get().getValue();
    var vb = bs.entrySet().stream().findFirst().get().getValue();
    return va.run() + vb.run();
  }

  // findAny().orElseThrow() — same Optional unwrap path.
  public static int useFindAnyOrElseThrow(Map<String, A> as, Map<String, B> bs) {
    return as.entrySet().stream().findAny().orElseThrow().getValue().run()
        + bs.entrySet().stream().findAny().orElseThrow().getValue().run();
  }

  // Optional.of(entry).get().getValue() — Entry local rewrapped.
  public static int useOptionalOfEntry(Map.Entry<String, A> ea, Map.Entry<String, B> eb) {
    return Optional.of(ea).get().getValue().run()
        + Optional.of(eb).get().getValue().run();
  }

  // Optional.of(entrySet iterator next).get().getValue() — nested Entry pipeline.
  public static int useOptionalOfIterNext(Map<String, A> as, Map<String, B> bs) {
    return Optional.of(as.entrySet().iterator().next()).get().getValue().run()
        + Optional.of(bs.entrySet().iterator().next()).get().getValue().run();
  }
}
