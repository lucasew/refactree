package demo;

import java.util.Collections;
import java.util.Map;
import java.util.stream.Collectors;
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
  public static int useMapOfGet() {
    var xa = Map.of("k", new A()).get("k");
    var xb = Map.of("k", new B()).get("k");
    return xa.run() + xb.run();
  }

  public static int useMapOfGetOrDefault() {
    var xa = Map.of("k", new A()).getOrDefault("k", new A());
    var xb = Map.of("k", new B()).getOrDefault("k", new B());
    return xa.run() + xb.run();
  }

  public static int useSingletonMapGet() {
    var xa = Collections.singletonMap("k", new A()).get("k");
    var xb = Collections.singletonMap("k", new B()).get("k");
    return xa.run() + xb.run();
  }

  public static int useMapOfEntriesGet() {
    var xa = Map.ofEntries(Map.entry("k", new A())).get("k");
    var xb = Map.ofEntries(Map.entry("k", new B())).get("k");
    return xa.run() + xb.run();
  }

  public static int useMapCopyOfGet(Map<String, A> as, Map<String, B> bs) {
    var xa = Map.copyOf(as).get("k");
    var xb = Map.copyOf(bs).get("k");
    return xa.run() + xb.run();
  }

  public static int useUnmodifiableMapGet(Map<String, A> as, Map<String, B> bs) {
    var xa = Collections.unmodifiableMap(as).get("k");
    var xb = Collections.unmodifiableMap(bs).get("k");
    return xa.run() + xb.run();
  }

  public static int useToMapCollectGet(Stream<A> as, Stream<B> bs) {
    var xa = as.collect(Collectors.toMap(a -> "k", a -> a)).get("k");
    var xb = bs.collect(Collectors.toMap(b -> "k", b -> b)).get("k");
    return xa.run() + xb.run();
  }
}
