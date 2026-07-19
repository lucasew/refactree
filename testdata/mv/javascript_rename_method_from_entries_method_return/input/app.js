class A {
  run() {
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

function useMember() {
  return (
    Object.fromEntries([["k", new BoxA().get()]]).k.run() +
    Object.fromEntries([["k", new BoxB().get()]]).k.run()
  );
}

function useBracket() {
  return (
    Object.fromEntries([["k", new BoxA().get()]])["k"].run() +
    Object.fromEntries([["k", new BoxB().get()]])["k"].run()
  );
}

function useLocal() {
  const oa = Object.fromEntries([["k", new BoxA().get()]]);
  const ob = Object.fromEntries([["k", new BoxB().get()]]);
  return oa.k.run() + ob.k.run();
}

function usePropAssign() {
  const xa = Object.fromEntries([["k", new BoxA().get()]]).k;
  const xb = Object.fromEntries([["k", new BoxB().get()]]).k;
  return xa.run() + xb.run();
}

function useValues() {
  return (
    Object.values(Object.fromEntries([["k", new BoxA().get()]]))[0].run() +
    Object.values(Object.fromEntries([["k", new BoxB().get()]]))[0].run()
  );
}

function useIdent() {
  const ba = new BoxA();
  const bb = new BoxB();
  return (
    Object.fromEntries([["k", ba.get()]]).k.run() +
    Object.fromEntries([["k", bb.get()]]).k.run()
  );
}

function usePreservesB() {
  const ob = Object.fromEntries([["k", new BoxB().get()]]);
  return (
    Object.fromEntries([["k", new BoxB().get()]]).k.run() +
    ob.k.run()
  );
}
