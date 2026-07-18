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

function useWeakMapGet() {
  const ka = {};
  const kb = {};
  return (
    new WeakMap([[ka, new A()]]).get(ka).execute() +
    new WeakMap([[kb, new B()]]).get(kb).run()
  );
}

function useWeakMapLocal() {
  const ka = {};
  const kb = {};
  const ma = new WeakMap([[ka, new A()]]);
  const mb = new WeakMap([[kb, new B()]]);
  return ma.get(ka).execute() + mb.get(kb).run();
}

function useWeakMapGetAssign() {
  const ka = {};
  const kb = {};
  const xa = new WeakMap([[ka, new A()]]).get(ka);
  const xb = new WeakMap([[kb, new B()]]).get(kb);
  return xa.execute() + xb.run();
}

function useMultiPair() {
  const ka = {};
  const ja = {};
  const kb = {};
  const jb = {};
  return (
    new WeakMap([
      [ka, new A()],
      [ja, new A()],
    ])
      .get(ja)
      .execute() +
    new WeakMap([
      [kb, new B()],
      [jb, new B()],
    ])
      .get(jb)
      .run()
  );
}

function useIdent() {
  const a = new A();
  const b = new B();
  const ka = {};
  const kb = {};
  return (
    new WeakMap([[ka, a]]).get(ka).execute() +
    new WeakMap([[kb, b]]).get(kb).run()
  );
}

function usePreservesB() {
  const k = {};
  const mb = new WeakMap([[k, new B()]]);
  return new WeakMap([[k, new B()]]).get(k).run() + mb.get(k).run();
}
