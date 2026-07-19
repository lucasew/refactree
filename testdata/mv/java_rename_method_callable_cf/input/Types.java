package demo;

import java.util.concurrent.Callable;
import java.util.concurrent.CompletableFuture;

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
  public static int useCallableCall(Callable<A> ca, Callable<B> cb) throws Exception {
    return ca.call().run() + cb.call().run();
  }

  public static int useCallableCallVar(Callable<A> ca, Callable<B> cb) throws Exception {
    var xa = ca.call();
    var xb = cb.call();
    return xa.run() + xb.run();
  }

  public static int useJoin(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.join().run() + fb.join().run();
  }

  public static int useGetNow(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.getNow(null).run() + fb.getNow(null).run();
  }

  public static int useResultNow(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    return fa.resultNow().run() + fb.resultNow().run();
  }

  public static int useJoinVar(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    var xa = fa.join();
    var xb = fb.join();
    return xa.run() + xb.run();
  }

  public static int useGetNowVar(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    var xa = fa.getNow(null);
    var xb = fb.getNow(null);
    return xa.run() + xb.run();
  }

  public static int useResultNowVar(CompletableFuture<A> fa, CompletableFuture<B> fb) {
    var xa = fa.resultNow();
    var xb = fb.resultNow();
    return xa.run() + xb.run();
  }

  public static int usePreservesB(Callable<B> cb, CompletableFuture<B> fb) throws Exception {
    return cb.call().run() + fb.join().run() + fb.getNow(null).run() + fb.resultNow().run();
  }
}
