package demo;

public class MyEx extends Exception {
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
  public static int use() {
    try {
      throw new MyEx();
    } catch (MyEx e) {
      return e.run();
    }
  }

  public static int useB() {
    B b = new B();
    return b.run();
  }
}
