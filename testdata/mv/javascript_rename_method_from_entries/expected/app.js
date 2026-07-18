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

function useFromEntriesMember() {
  return (
    Object.fromEntries([["k", new A()]]).k.execute() +
    Object.fromEntries([["k", new B()]]).k.run()
  );
}

function useFromEntriesBracket() {
  return (
    Object.fromEntries([["k", new A()]])["k"].execute() +
    Object.fromEntries([["k", new B()]])["k"].run()
  );
}

function useFromEntriesLocal() {
  const oa = Object.fromEntries([["k", new A()]]);
  const ob = Object.fromEntries([["k", new B()]]);
  return oa.k.execute() + ob.k.run();
}

function useFromEntriesLocalBracket() {
  const oa = Object.fromEntries([["k", new A()]]);
  const ob = Object.fromEntries([["k", new B()]]);
  return oa["k"].execute() + ob["k"].run();
}

function useFromEntriesPropAssign() {
  const xa = Object.fromEntries([["k", new A()]]).k;
  const xb = Object.fromEntries([["k", new B()]]).k;
  return xa.execute() + xb.run();
}

function useFromEntriesValues() {
  return (
    Object.values(Object.fromEntries([["k", new A()]]))[0].execute() +
    Object.values(Object.fromEntries([["k", new B()]]))[0].run()
  );
}

function useFromEntriesPairsLocal() {
  const pa = [["k", new A()]];
  const pb = [["k", new B()]];
  return (
    Object.fromEntries(pa).k.execute() + Object.fromEntries(pb).k.run()
  );
}

function useMultiPair() {
  return (
    Object.fromEntries([
      ["k", new A()],
      ["j", new A()],
    ]).j.execute() +
    Object.fromEntries([
      ["k", new B()],
      ["j", new B()],
    ]).j.run()
  );
}

function useIdent() {
  const a = new A();
  const b = new B();
  return (
    Object.fromEntries([["k", a]]).k.execute() +
    Object.fromEntries([["k", b]]).k.run()
  );
}

function usePreservesB() {
  const ob = Object.fromEntries([["k", new B()]]);
  return (
    Object.fromEntries([["k", new B()]]).k.run() +
    ob.k.run() +
    Object.values(Object.fromEntries([["k", new B()]]))[0].run()
  );
}
