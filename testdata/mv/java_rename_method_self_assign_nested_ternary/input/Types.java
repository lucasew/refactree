package demo;

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

class BoxA {
  A a = new A();

  A get() {
    return a;
  }

  BoxA self() {
    return this;
  }
}

class BoxB {
  B b = new B();

  B get() {
    return b;
  }

  BoxB self() {
    return this;
  }
}

class HolderA {
  BoxA box = new BoxA();
}

class HolderB {
  BoxB box = new BoxB();
}

class OuterA {
  HolderA h = new HolderA();
}

class OuterB {
  HolderB h = new HolderB();
}

class Uses {
  public static int useSelfAssign(BoxA ba, BoxB bb) {
    var xa = ba.self();
    var xb = bb.self();
    return xa.get().run() + xb.get().run();
  }

  public static int useNestedField(OuterA oa, OuterB ob) {
    return oa.h.box.get().run() + ob.h.box.get().run();
  }

  public static int useTernary(boolean c, BoxA ba, BoxB bb) {
    return (c ? ba.get() : ba.get()).run() + (c ? bb.get() : bb.get()).run();
  }

  public static int usePreservesB(BoxB bb, OuterB ob) {
    var xb = bb.self();
    return xb.get().run() + ob.h.box.get().run();
  }
}
