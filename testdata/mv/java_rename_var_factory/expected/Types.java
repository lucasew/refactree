package demo;

public class A {
  public int execute() {
    return 1;
  }

  public static A make() {
    return new A();
  }

  public static A getInstance() {
    return new A();
  }
}

class B {
  public int run() {
    return 2;
  }

  public static B make() {
    return new B();
  }
}

class Uses {
  public static int useMake() {
    var a = A.make();
    return a.execute();
  }

  public static int useGetInstance() {
    var a = A.getInstance();
    return a.execute();
  }

  public static int useParen() {
    var a = (A.make());
    return a.execute();
  }

  public static int useCast() {
    var a = (A) A.make();
    return a.execute();
  }

  public static int useB() {
    var b = B.make();
    return b.run();
  }

  public static int useNewStill() {
    var a = new A();
    return a.execute();
  }
}
