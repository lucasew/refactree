package demo;

public class A {
  public int run() {
    return 1;
  }

  public static A make() {
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
  public static int useTernaryNew(boolean f) {
    var tNew = f ? new A() : new A();
    return tNew.run();
  }

  public static int useTernaryB(boolean f) {
    var tB = f ? new B() : new B();
    return tB.run();
  }

  public static int useTernaryFactory(boolean f) {
    var tFac = f ? A.make() : A.make();
    return tFac.run();
  }

  public static int useTernaryMixed(boolean f, A known) {
    // both arms type as A via inferable shapes
    var tMix = f ? new A() : A.make();
    return tMix.run();
  }

  public static int useSwitchNew(int x) {
    var sNew = switch (x) {
      case 0 -> new A();
      default -> new A();
    };
    return sNew.run();
  }

  public static int useSwitchB(int x) {
    var sB = switch (x) {
      case 0 -> new B();
      default -> new B();
    };
    return sB.run();
  }

  public static int useSwitchFactory(int x) {
    var sFac = switch (x) {
      case 0 -> A.make();
      default -> A.make();
    };
    return sFac.run();
  }

  public static int useSwitchYield(int x) {
    var sY = switch (x) {
      case 0: yield new A();
      default: yield A.make();
    };
    return sY.run();
  }
}
