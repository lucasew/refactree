package demo;

import java.util.function.Function;

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
  public static int useIdentity() {
    return Function.identity().apply(new A()).execute()
        + Function.identity().apply(new B()).run();
  }

  public static int useTypedIdentity() {
    return Function.<A>identity().apply(new A()).execute()
        + Function.<B>identity().apply(new B()).run();
  }

  public static int useIdentityLocal(A a, B b) {
    return Function.identity().apply(a).execute()
        + Function.identity().apply(b).run();
  }

  public static int useIdentityAssign() {
    var xa = Function.identity().apply(new A());
    var xb = Function.identity().apply(new B());
    return xa.execute() + xb.run();
  }

  public static int usePreservesB() {
    return Function.identity().apply(new B()).run();
  }
}
