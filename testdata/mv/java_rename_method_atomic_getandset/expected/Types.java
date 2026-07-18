package demo;

import java.util.concurrent.atomic.AtomicReference;
import java.util.function.BinaryOperator;
import java.util.function.UnaryOperator;

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
  public static int useGetAndSet(AtomicReference<A> aa, AtomicReference<B> ab) {
    return aa.getAndSet(null).execute() + ab.getAndSet(null).run();
  }

  public static int useGetAndSetVar(AtomicReference<A> aa, AtomicReference<B> ab) {
    var xa = aa.getAndSet(null);
    var xb = ab.getAndSet(null);
    return xa.execute() + xb.run();
  }

  public static int useGetAndUpdate(AtomicReference<A> aa, AtomicReference<B> ab, UnaryOperator<A> ua, UnaryOperator<B> ub) {
    return aa.getAndUpdate(ua).execute() + ab.getAndUpdate(ub).run();
  }

  public static int useUpdateAndGet(AtomicReference<A> aa, AtomicReference<B> ab, UnaryOperator<A> ua, UnaryOperator<B> ub) {
    return aa.updateAndGet(ua).execute() + ab.updateAndGet(ub).run();
  }

  public static int useAccumulateAndGet(AtomicReference<A> aa, AtomicReference<B> ab, BinaryOperator<A> oa, BinaryOperator<B> ob) {
    return aa.accumulateAndGet(null, oa).execute() + ab.accumulateAndGet(null, ob).run();
  }

  public static int useCompareAndExchange(AtomicReference<A> aa, AtomicReference<B> ab) {
    return aa.compareAndExchange(null, null).execute() + ab.compareAndExchange(null, null).run();
  }

  public static int useGetPlain(AtomicReference<A> aa, AtomicReference<B> ab) {
    return aa.getPlain().execute() + ab.getPlain().run();
  }

  public static int useGetAcquire(AtomicReference<A> aa, AtomicReference<B> ab) {
    return aa.getAcquire().execute() + ab.getAcquire().run();
  }

  public static int useGetOpaque(AtomicReference<A> aa, AtomicReference<B> ab) {
    return aa.getOpaque().execute() + ab.getOpaque().run();
  }

  public static int usePreservesB(AtomicReference<B> ab, UnaryOperator<B> ub) {
    return ab.getAndSet(null).run()
        + ab.getAndUpdate(ub).run()
        + ab.updateAndGet(ub).run()
        + ab.getPlain().run()
        + ab.getAcquire().run()
        + ab.getOpaque().run();
  }
}
