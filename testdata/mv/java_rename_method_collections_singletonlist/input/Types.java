package demo;

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
  public static int useSingletonListForEach() {
    Collections.singletonList(new A()).forEach(a -> a.run());
    Collections.singletonList(new B()).forEach(b -> b.run());
    return 0;
  }

  public static int useVarSingletonList() {
    var al = Collections.singletonList(new A());
    var bl = Collections.singletonList(new B());
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

  public static int useSingletonListFor() {
    int n = 0;
    for (var a : Collections.singletonList(new A())) {
      n += a.run();
    }
    for (var b : Collections.singletonList(new B())) {
      n += b.run();
    }
    return n;
  }
}
