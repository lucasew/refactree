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

record BoxA(A a) {
  A get() {
    return a;
  }

  BoxA self() {
    return this;
  }
}

record BoxB(B b) {
  B get() {
    return b;
  }

  BoxB self() {
    return this;
  }
}

class HolderA {
  A item = new A();

  A get() {
    return item;
  }
}

class HolderB {
  B item = new B();

  B get() {
    return item;
  }
}

class Uses {
  public static int useGetter(BoxA ba, BoxB bb) {
    return ba.get().run() + bb.get().run();
  }

  public static int useSelf(BoxA ba, BoxB bb) {
    return ba.self().a().run() + bb.self().b().run();
  }

  public static int useSelfGet(BoxA ba, BoxB bb) {
    return ba.self().get().run() + bb.self().get().run();
  }

  public static int useHolder(HolderA ha, HolderB hb) {
    return ha.get().run() + hb.get().run();
  }

  public static int usePreservesB(BoxB bb, HolderB hb) {
    return bb.get().run() + bb.self().b().run() + hb.get().run();
  }
}
