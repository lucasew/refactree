package demo;

import java.util.Optional;

public class A {
  public int execute() {
    return 1;
  }

  public static A create() {
    return new A();
  }
}

class B {
  public int run() {
    return 2;
  }

  public static B create() {
    return new B();
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

class OuterA {
  HolderA h = new HolderA();
}

class OuterB {
  HolderB h = new HolderB();
}

class BoxA {
  A a;

  A get() {
    return a;
  }
}

class BoxB {
  B b;

  B get() {
    return b;
  }
}

class Uses {
  public static int useField(OuterA oa, OuterB ob) {
    return oa.h.get().execute() + ob.h.get().run();
  }

  public static int useFieldAssign(OuterA oa, OuterB ob) {
    var ha = oa.h;
    var hb = ob.h;
    return ha.get().execute() + hb.get().run();
  }

  public static int useStatic() {
    return A.create().execute() + B.create().run();
  }

  public static int useStaticAssign() {
    var a = A.create();
    var b = B.create();
    return a.execute() + b.run();
  }

  public static int useOptional(BoxA ba, BoxB bb) {
    return Optional.of(ba.get()).get().execute() + Optional.of(bb.get()).get().run();
  }

  public static int useOptionalAssign(BoxA ba, BoxB bb) {
    var xa = Optional.of(ba.get()).get();
    var xb = Optional.of(bb.get()).get();
    return xa.execute() + xb.run();
  }

  public static int usePreservesB(OuterB ob, BoxB bb) {
    return ob.h.get().run() + B.create().run() + Optional.of(bb.get()).get().run();
  }
}
