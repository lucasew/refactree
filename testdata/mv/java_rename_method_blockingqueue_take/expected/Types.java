package demo;

import java.util.concurrent.BlockingDeque;
import java.util.concurrent.BlockingQueue;

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
  public static int useTake(BlockingQueue<A> qa, BlockingQueue<B> qb) throws InterruptedException {
    var xa = qa.take();
    var xb = qb.take();
    return xa.execute() + xb.run();
  }

  public static int useTakeFirst(BlockingDeque<A> da, BlockingDeque<B> db) throws InterruptedException {
    var xa = da.takeFirst();
    var xb = db.takeFirst();
    return xa.execute() + xb.run();
  }

  public static int useTakeLast(BlockingDeque<A> da, BlockingDeque<B> db) throws InterruptedException {
    var ya = da.takeLast();
    var yb = db.takeLast();
    return ya.execute() + yb.run();
  }
}
