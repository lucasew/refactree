package demo;

import java.util.function.BiFunction;
import java.util.function.Function;
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
  public static int useFunctionApply(Function<String, A> fa, Function<String, B> fb) {
    return fa.apply("k").execute() + fb.apply("k").run();
  }

  public static int useFunctionApplyVar(Function<String, A> fa, Function<String, B> fb) {
    var xa = fa.apply("k");
    var xb = fb.apply("k");
    return xa.execute() + xb.run();
  }

  public static int useUnaryApply(UnaryOperator<A> ua, UnaryOperator<B> ub) {
    return ua.apply(null).execute() + ub.apply(null).run();
  }

  public static int useUnaryApplyVar(UnaryOperator<A> ua, UnaryOperator<B> ub) {
    var xa = ua.apply(null);
    var xb = ub.apply(null);
    return xa.execute() + xb.run();
  }

  public static int useBiFunctionApply(BiFunction<String, Integer, A> bfa, BiFunction<String, Integer, B> bfb) {
    return bfa.apply("k", 1).execute() + bfb.apply("k", 1).run();
  }

  public static int useBiFunctionApplyVar(BiFunction<String, Integer, A> bfa, BiFunction<String, Integer, B> bfb) {
    var xa = bfa.apply("k", 1);
    var xb = bfb.apply("k", 1);
    return xa.execute() + xb.run();
  }

  public static int usePreservesB(Function<String, B> fb, UnaryOperator<B> ub, BiFunction<String, Integer, B> bfb) {
    return fb.apply("k").run() + ub.apply(null).run() + bfb.apply("k", 1).run();
  }
}
