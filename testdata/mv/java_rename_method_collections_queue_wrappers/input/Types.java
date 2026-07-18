package demo;

import java.util.Collections;
import java.util.Deque;
import java.util.Queue;

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
  // Chain: Collections asLifoQueue/checkedQueue are element-type-preserving
  // (same path as unmodifiableSet / checkedList; Class args ignored).
  public static int useAsLifoQueuePoll(Deque<A> as, Deque<B> bs) {
    return Collections.asLifoQueue(as).poll().run()
        + Collections.asLifoQueue(bs).peek().run();
  }

  public static int useCheckedQueuePoll(Queue<A> as, Queue<B> bs) {
    return Collections.checkedQueue(as, A.class).poll().run()
        + Collections.checkedQueue(bs, B.class).element().run();
  }

  // var from wrapper then poll/peek — element leaf through elemOf.
  public static int useVarAsLifoQueue(Deque<A> as, Deque<B> bs) {
    var al = Collections.asLifoQueue(as);
    var bl = Collections.asLifoQueue(bs);
    var xa = al.poll();
    var xb = bl.peek();
    return xa.run() + xb.run();
  }

  public static int useVarCheckedQueue(Queue<A> as, Queue<B> bs) {
    var al = Collections.checkedQueue(as, A.class);
    var bl = Collections.checkedQueue(bs, B.class);
    return al.poll().run() + bl.element().run();
  }

  // forEach / for-in through wrapper (neighbor paths).
  public static int useWrapperForEach(Deque<A> as, Deque<B> bs) {
    Collections.asLifoQueue(as).forEach(a -> a.run());
    Collections.checkedQueue(bs, B.class).forEach(b -> b.run());
    return 0;
  }

  public static int useWrapperFor(Deque<A> as, Queue<B> bs) {
    int n = 0;
    for (var a : Collections.asLifoQueue(as)) {
      n += a.run();
    }
    for (var b : Collections.checkedQueue(bs, B.class)) {
      n += b.run();
    }
    return n;
  }
}
