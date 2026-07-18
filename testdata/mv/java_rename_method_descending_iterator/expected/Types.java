package demo;

import java.util.Deque;
import java.util.Iterator;
import java.util.NavigableSet;

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
  // Chain: descendingIterator is type-preserving like iterator/listIterator.
  public static int useDescIterChain(Deque<A> as, Deque<B> bs) {
    return as.descendingIterator().next().execute() + bs.descendingIterator().next().run();
  }

  // var from descendingIterator().next()
  public static int useDescIterVar(Deque<A> as, Deque<B> bs) {
    var xa = as.descendingIterator().next();
    var xb = bs.descendingIterator().next();
    return xa.execute() + xb.run();
  }

  // NavigableSet.descendingIterator
  public static int useNavSetDescChain(NavigableSet<A> as, NavigableSet<B> bs) {
    return as.descendingIterator().next().execute() + bs.descendingIterator().next().run();
  }

  public static int useNavSetDescVar(NavigableSet<A> as, NavigableSet<B> bs) {
    var ya = as.descendingIterator().next();
    var yb = bs.descendingIterator().next();
    return ya.execute() + yb.run();
  }

  // Typed Iterator local from descendingIterator (explicit type path).
  public static int useDescIterTypedLocal(Deque<A> as, Deque<B> bs) {
    Iterator<A> ia = as.descendingIterator();
    Iterator<B> ib = bs.descendingIterator();
    var za = ia.next();
    var zb = ib.next();
    return za.execute() + zb.run();
  }

  // Regression: plain iterator still works.
  public static int useIterChain(Deque<A> as, Deque<B> bs) {
    return as.iterator().next().execute() + bs.iterator().next().run();
  }
}
