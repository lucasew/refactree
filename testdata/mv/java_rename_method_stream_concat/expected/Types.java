package demo;

import java.util.List;
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
  public static int useConcatForEach(List<A> as, List<B> bs) {
    Stream.concat(as.stream(), as.stream()).forEach(a -> a.execute());
    Stream.concat(bs.stream(), bs.stream()).forEach(b -> b.run());
    return 0;
  }

  public static int useConcatStreamOf(List<A> as, List<B> bs) {
    Stream.concat(as.stream(), Stream.of(new A())).forEach(a -> a.execute());
    Stream.concat(bs.stream(), Stream.of(new B())).forEach(b -> b.run());
    return 0;
  }

  public static int useConcatToListFor(List<A> as, List<B> bs) {
    int n = 0;
    for (var a : Stream.concat(as.stream(), as.stream()).toList()) {
      n += a.execute();
    }
    for (var b : Stream.concat(bs.stream(), bs.stream()).toList()) {
      n += b.run();
    }
    return n;
  }

  public static int useVarConcat(List<A> as, List<B> bs) {
    var sa = Stream.concat(as.stream(), as.stream());
    var sb = Stream.concat(bs.stream(), bs.stream());
    sa.forEach(a -> a.execute());
    sb.forEach(b -> b.run());
    int n = 0;
    for (var a : sa.toList()) {
      n += a.execute();
    }
    for (var b : sb.toList()) {
      n += b.run();
    }
    return n;
  }
}
