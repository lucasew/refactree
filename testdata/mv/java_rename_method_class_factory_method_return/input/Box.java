class A {
  int run() {
    return 1;
  }

  static A make() {
    return new A();
  }

  static A fromBox(BoxA ba) {
    return ba.get();
  }

  static A fromBoxAssign(BoxA ba) {
    A cxa = ba.get();
    return cxa;
  }

  A fromBoxI(BoxA ba) {
    return ba.get();
  }
}

class B {
  int run() {
    return 2;
  }

  static B make() {
    return new B();
  }

  static B fromBox(BoxB bb) {
    return bb.get();
  }

  static B fromBoxAssign(BoxB bb) {
    B cxb = bb.get();
    return cxb;
  }

  B fromBoxI(BoxB bb) {
    return bb.get();
  }
}

class BoxA {
  A a = new A();

  A get() {
    return a;
  }
}

class BoxB {
  B b = new B();

  B get() {
    return b;
  }
}

class Use {
  // Class static factories (should solid).
  int useClassStatic() {
    return A.make().run() + B.make().run();
  }

  int useClassStaticAssign() {
    A csa = A.make();
    B csb = B.make();
    return csa.run() + csb.run();
  }

  // Method-return class factories (likely UNDER).
  int useMRStatic(BoxA ba, BoxB bb) {
    return A.fromBox(ba).run() + B.fromBox(bb).run();
  }

  int useMRStaticAssign(BoxA ba, BoxB bb) {
    A msa = A.fromBox(ba);
    B msb = B.fromBox(bb);
    return msa.run() + msb.run();
  }

  int useMRStaticBodyAssign(BoxA ba, BoxB bb) {
    return A.fromBoxAssign(ba).run() + B.fromBoxAssign(bb).run();
  }

  int useMRInstance(A ia, B ib, BoxA ba, BoxB bb) {
    return ia.fromBoxI(ba).run() + ib.fromBoxI(bb).run();
  }

  int useMRInstanceAssign(A ia, B ib, BoxA ba, BoxB bb) {
    A mia = ia.fromBoxI(ba);
    B mib = ib.fromBoxI(bb);
    return mia.run() + mib.run();
  }

  int useMRInstanceNew(BoxA ba, BoxB bb) {
    return new A().fromBoxI(ba).run() + new B().fromBoxI(bb).run();
  }

  // Preserves B.
  int usePreservesB(BoxB bb) {
    return B.fromBox(bb).run() + B.make().run();
  }
}
