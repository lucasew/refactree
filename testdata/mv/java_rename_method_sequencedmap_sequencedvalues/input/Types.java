package demo;

import java.util.LinkedHashMap;
import java.util.SequencedMap;

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
  // sequencedValues().getFirst() — V under foreign same-leaf (Java 21).
  public static int useGetFirst(SequencedMap<String, A> as, SequencedMap<String, B> bs) {
    return as.sequencedValues().getFirst().run()
        + bs.sequencedValues().getFirst().run();
  }

  // sequencedValues().getLast() — same V leaf.
  public static int useGetLast(SequencedMap<String, A> as, SequencedMap<String, B> bs) {
    return as.sequencedValues().getLast().run()
        + bs.sequencedValues().getLast().run();
  }

  // var from sequencedValues getFirst.
  public static int useVarGetFirst(SequencedMap<String, A> as, SequencedMap<String, B> bs) {
    var va = as.sequencedValues().getFirst();
    var vb = bs.sequencedValues().getFirst();
    return va.run() + vb.run();
  }

  // sequencedValues forEach / for-var.
  public static int useForEach(SequencedMap<String, A> as, SequencedMap<String, B> bs) {
    as.sequencedValues().forEach(a -> a.run());
    bs.sequencedValues().forEach(b -> b.run());
    int n = 0;
    for (var a : as.sequencedValues()) {
      n += a.run();
    }
    for (var b : bs.sequencedValues()) {
      n += b.run();
    }
    return n;
  }

  // LinkedHashMap concrete + reversed().sequencedValues().
  public static int useLinkedHashMap(LinkedHashMap<String, A> as, LinkedHashMap<String, B> bs) {
    return as.sequencedValues().getFirst().run()
        + bs.sequencedValues().getFirst().run()
        + as.reversed().sequencedValues().getFirst().run()
        + bs.reversed().sequencedValues().getFirst().run();
  }
}
