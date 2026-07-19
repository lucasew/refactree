package demo;

import java.util.function.Function;

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
  public static int useIdentity() {
    return Function.identity().apply(new A()).run()
        + Function.identity().apply(new B()).run();
  }

  public static int useTypedIdentity() {
    return Function.<A>identity().apply(new A()).run()
        + Function.<B>identity().apply(new B()).run();
  }

  public static int useIdentityLocal(A a, B b) {
    return Function.identity().apply(a).run()
        + Function.identity().apply(b).run();
  }

  public static int useIdentityAssign() {
    var xa = Function.identity().apply(new A());
    var xb = Function.identity().apply(new B());
    return xa.run() + xb.run();
  }

  public static int usePreservesB() {
    return Function.identity().apply(new B()).run();
  }
}
