package demo;

import java.util.Stack;

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
  public static int usePush(Stack<A> as, Stack<B> bs) {
    var xa = as.push(new A());
    var xb = bs.push(new B());
    return xa.run() + xb.run();
  }
}
