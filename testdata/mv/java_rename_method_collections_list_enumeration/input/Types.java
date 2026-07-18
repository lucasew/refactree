package demo;

import java.util.Collections;
import java.util.Enumeration;
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
  public static int useListEnumForEach(List<A> as, List<B> bs) {
    Collections.list(Collections.enumeration(as)).forEach(a -> a.run());
    Collections.list(Collections.enumeration(bs)).forEach(b -> b.run());
    return 0;
  }

  public static int useTypedEnumForEach(Enumeration<A> ea, Enumeration<B> eb) {
    Collections.list(ea).forEach(a -> a.run());
    Collections.list(eb).forEach(b -> b.run());
    return 0;
  }

  public static int useVarList(List<A> as, List<B> bs) {
    var al = Collections.list(Collections.enumeration(as));
    var bl = Collections.list(Collections.enumeration(bs));
    al.forEach(a -> a.run());
    bl.forEach(b -> b.run());
    int n = 0;
    for (var a : al) {
      n += a.run();
    }
    for (var b : bl) {
      n += b.run();
    }
    return n;
  }

  public static int useListFor(List<A> as, List<B> bs) {
    int n = 0;
    for (var a : Collections.list(Collections.enumeration(as))) {
      n += a.run();
    }
    for (var b : Collections.list(Collections.enumeration(bs))) {
      n += b.run();
    }
    return n;
  }

  public static int useTypedEnumVar(Enumeration<A> ea, Enumeration<B> eb) {
    var al = Collections.list(ea);
    var bl = Collections.list(eb);
    al.forEach(a -> a.run());
    bl.forEach(b -> b.run());
    return 0;
  }
}
