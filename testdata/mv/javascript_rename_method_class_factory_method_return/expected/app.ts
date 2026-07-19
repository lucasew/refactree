class A {
  execute(): number {
    return 1;
  }
  static make(): A {
    return new A();
  }
  static create(): A {
    return new A();
  }
  // Method-return factories: peel ba.get() via typed param / body assign.
  static fromBox(ba: BoxA): A {
    return ba.get();
  }
  static fromBoxAssign(ba: BoxA): A {
    const cxa = ba.get();
    return cxa;
  }
  fromBoxI(ba: BoxA): A {
    return ba.get();
  }
  // Zero-arg method-return factory (new BoxA().get()).
  static fromNew(): A {
    return new BoxA().get();
  }
}

class B {
  run(): number {
    return 2;
  }
  static make(): B {
    return new B();
  }
  static create(): B {
    return new B();
  }
  static fromBox(bb: BoxB): B {
    return bb.get();
  }
  static fromBoxAssign(bb: BoxB): B {
    const cxb = bb.get();
    return cxb;
  }
  fromBoxI(bb: BoxB): B {
    return bb.get();
  }
  static fromNew(): B {
    return new BoxB().get();
  }
}

class BoxA {
  a: A = new A();
  get(): A {
    return this.a;
  }
}

class BoxB {
  b: B = new B();
  get(): B {
    return this.b;
  }
}

// --- Class static factories (already solid). ---
function useClassStatic(): number {
  return A.make().execute() + B.make().run();
}

function useClassStaticAssign(): number {
  const csa = A.make();
  const csb = B.make();
  return csa.execute() + csb.run();
}

function useClassCreate(): number {
  return A.create().execute() + B.create().run();
}

// --- Method-return class factories (were UNDER). ---
function useMRStatic(ba: BoxA, bb: BoxB): number {
  return A.fromBox(ba).execute() + B.fromBox(bb).run();
}

function useMRStaticAssign(ba: BoxA, bb: BoxB): number {
  const msa = A.fromBox(ba);
  const msb = B.fromBox(bb);
  return msa.execute() + msb.run();
}

function useMRStaticBodyAssign(ba: BoxA, bb: BoxB): number {
  return A.fromBoxAssign(ba).execute() + B.fromBoxAssign(bb).run();
}

function useMRFromNew(): number {
  return A.fromNew().execute() + B.fromNew().run();
}

function useMRInstance(ia: A, ib: B, ba: BoxA, bb: BoxB): number {
  return ia.fromBoxI(ba).execute() + ib.fromBoxI(bb).run();
}

function useMRInstanceAssign(ia: A, ib: B, ba: BoxA, bb: BoxB): number {
  const mia = ia.fromBoxI(ba);
  const mib = ib.fromBoxI(bb);
  return mia.execute() + mib.run();
}

function useMRInstanceNew(ba: BoxA, bb: BoxB): number {
  return new A().fromBoxI(ba).execute() + new B().fromBoxI(bb).run();
}

// Preserves B under foreign same-leaf.
function usePreservesB(bb: BoxB): number {
  return B.fromBox(bb).run() + B.make().run() + B.fromNew().run();
}
