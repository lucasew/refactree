package demo;

import java.util.Deque;
import java.util.List;
import java.util.Map;
import java.util.Queue;

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
  public static int usePoll(Queue<A> qa, Queue<B> qb) {
    var xa = qa.poll();
    var xb = qb.poll();
    return xa.execute() + xb.run();
  }

  public static int usePeek(Queue<A> qa, Queue<B> qb) {
    var xa = qa.peek();
    var xb = qb.peek();
    return xa.execute() + xb.run();
  }

  public static int useElement(Queue<A> qa, Queue<B> qb) {
    var xa = qa.element();
    var xb = qb.element();
    return xa.execute() + xb.run();
  }

  public static int useRemove(List<A> as, List<B> bs) {
    var xa = as.remove(0);
    var xb = bs.remove(0);
    return xa.execute() + xb.run();
  }

  public static int useMapRemove(Map<String, A> am, Map<String, B> bm) {
    var xa = am.remove("k");
    var xb = bm.remove("k");
    return xa.execute() + xb.run();
  }

  public static int useGetFirst(List<A> as, List<B> bs) {
    var xa = as.getFirst();
    var xb = bs.getFirst();
    return xa.execute() + xb.run();
  }

  public static int useGetLast(List<A> as, List<B> bs) {
    var xa = as.getLast();
    var xb = bs.getLast();
    return xa.execute() + xb.run();
  }

  public static int useDeque(Deque<A> da, Deque<B> db) {
    var xa = da.pollFirst();
    var xb = db.peekLast();
    var ya = da.pop();
    var yb = db.pop();
    return xa.execute() + xb.run() + ya.execute() + yb.run();
  }
}
