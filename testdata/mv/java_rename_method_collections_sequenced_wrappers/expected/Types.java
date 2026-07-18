package demo;

import java.util.Collections;
import java.util.SequencedCollection;
import java.util.SequencedMap;
import java.util.SequencedSet;

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
  // Chain: Collections SequencedCollection/Set wrappers are element-type-preserving
  // (same path as unmodifiableSet / unmodifiableNavigableSet; Java 21).
  public static int useUnmodifiableSequencedCollection(
      SequencedCollection<A> as, SequencedCollection<B> bs) {
    return Collections.unmodifiableSequencedCollection(as).getFirst().execute()
        + Collections.unmodifiableSequencedCollection(bs).getLast().run();
  }

  public static int useSynchronizedSequencedCollection(
      SequencedCollection<A> as, SequencedCollection<B> bs) {
    return Collections.synchronizedSequencedCollection(as).getFirst().execute()
        + Collections.synchronizedSequencedCollection(bs).getLast().run();
  }

  public static int useUnmodifiableSequencedSet(
      SequencedSet<A> as, SequencedSet<B> bs) {
    return Collections.unmodifiableSequencedSet(as).getFirst().execute()
        + Collections.unmodifiableSequencedSet(bs).getLast().run();
  }

  public static int useSynchronizedSequencedSet(
      SequencedSet<A> as, SequencedSet<B> bs) {
    return Collections.synchronizedSequencedSet(as).getFirst().execute()
        + Collections.synchronizedSequencedSet(bs).getLast().run();
  }

  // SequencedMap wrappers — value type preserving (mirrors unmodifiableNavigableMap).
  public static int useUnmodifiableSequencedMapGet(
      SequencedMap<String, A> am, SequencedMap<String, B> bm) {
    return Collections.unmodifiableSequencedMap(am).get("k").execute()
        + Collections.unmodifiableSequencedMap(bm).get("k").run();
  }

  public static int useSynchronizedSequencedMapGet(
      SequencedMap<String, A> am, SequencedMap<String, B> bm) {
    return Collections.synchronizedSequencedMap(am).get("k").execute()
        + Collections.synchronizedSequencedMap(bm).get("k").run();
  }

  public static int useUnmodifiableSequencedMapFirstEntry(
      SequencedMap<String, A> am, SequencedMap<String, B> bm) {
    return Collections.unmodifiableSequencedMap(am).firstEntry().getValue().execute()
        + Collections.unmodifiableSequencedMap(bm).lastEntry().getValue().run();
  }

  // var from wrapper then endpoint — element/value leaf through elemOf/valOf.
  public static int useVarSequencedCollection(
      SequencedCollection<A> as, SequencedCollection<B> bs) {
    var al = Collections.unmodifiableSequencedCollection(as);
    var bl = Collections.synchronizedSequencedCollection(bs);
    var xa = al.getFirst();
    var xb = bl.getLast();
    return xa.execute() + xb.run();
  }

  public static int useVarSequencedSet(SequencedSet<A> as, SequencedSet<B> bs) {
    var al = Collections.unmodifiableSequencedSet(as);
    var bl = Collections.synchronizedSequencedSet(bs);
    return al.getFirst().execute() + bl.getLast().run();
  }

  public static int useVarSequencedMap(
      SequencedMap<String, A> am, SequencedMap<String, B> bm) {
    var al = Collections.unmodifiableSequencedMap(am);
    var bl = Collections.synchronizedSequencedMap(bm);
    var xa = al.get("k");
    var xb = bl.get("k");
    return xa.execute() + xb.run();
  }

  // forEach / for-in through wrapper (neighbor paths).
  public static int useWrapperForEach(
      SequencedCollection<A> as, SequencedCollection<B> bs) {
    Collections.unmodifiableSequencedCollection(as).forEach(a -> a.execute());
    Collections.unmodifiableSequencedCollection(bs).forEach(b -> b.run());
    return 0;
  }

  public static int useWrapperForIn(SequencedSet<A> as, SequencedSet<B> bs) {
    int n = 0;
    for (var a : Collections.unmodifiableSequencedSet(as)) {
      n += a.execute();
    }
    for (var b : Collections.unmodifiableSequencedSet(bs)) {
      n += b.run();
    }
    return n;
  }

  public static int useMapValuesForEach(
      SequencedMap<String, A> am, SequencedMap<String, B> bm) {
    Collections.unmodifiableSequencedMap(am).values().forEach(a -> a.execute());
    Collections.unmodifiableSequencedMap(bm).values().forEach(b -> b.run());
    return 0;
  }

  // Regression: plain unmodifiableCollection / unmodifiableMap already worked.
  public static int usePlainUnmodifiableCollection(
      SequencedCollection<A> as, SequencedCollection<B> bs) {
    Collections.unmodifiableCollection(as).forEach(a -> a.execute());
    Collections.unmodifiableCollection(bs).forEach(b -> b.run());
    return 0;
  }

  public static int usePlainUnmodifiableMapGet(
      SequencedMap<String, A> am, SequencedMap<String, B> bm) {
    return Collections.unmodifiableMap(am).get("k").execute()
        + Collections.unmodifiableMap(bm).get("k").run();
  }

  public static int usePlainGetFirst(
      SequencedCollection<A> as, SequencedCollection<B> bs) {
    var xa = as.getFirst();
    var xb = bs.getLast();
    return xa.execute() + xb.run();
  }
}
