class A {
  execute() {
    return 1;
  }
}
class B {
  run() {
    return 2;
  }
}
class BoxA {
  a = new A();
  get() {
    return this.a;
  }
}
class BoxB {
  b = new B();
  get() {
    return this.b;
  }
}

// Class nullish/logical assign (was UNDER).
function useClassNullish() {
  let csa = null;
  csa ??= new A();
  let csb = null;
  csb ??= new B();
  return csa.execute() + csb.run();
}

function useClassOr() {
  let coa = null;
  coa ||= new A();
  let cob = null;
  cob ||= new B();
  return coa.execute() + cob.run();
}

// Method-return nullish/logical assign (was UNDER).
function useMRNullish() {
  let msa = null;
  msa ??= new BoxA().get();
  let msb = null;
  msb ??= new BoxB().get();
  return msa.execute() + msb.run();
}

function useMROr() {
  let moa = null;
  moa ||= new BoxA().get();
  let mob = null;
  mob ||= new BoxB().get();
  return moa.execute() + mob.run();
}

function usePreservesB() {
  let msb = null;
  msb ??= new BoxB().get();
  return msb.run();
}
