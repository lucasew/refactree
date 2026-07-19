package demo;

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

record BoxA(A a) {}
record BoxB(B b) {}

class Uses {
  public static int useInline() {
    return new BoxA(new A()).a().execute() + new BoxB(new B()).b().run();
  }

  public static int useVar() {
    var xa = new BoxA(new A()).a();
    var xb = new BoxB(new B()).b();
    return xa.execute() + xb.run();
  }

  public static int useAssigned() {
    var ba = new BoxA(new A());
    var bb = new BoxB(new B());
    return ba.a().execute() + bb.b().run();
  }

  public static int usePreservesB() {
    return new BoxB(new B()).b().run();
  }
}
