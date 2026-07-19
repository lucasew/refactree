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

function useFromEntriesMapLocal() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  return Object.fromEntries(ma).k.execute() + Object.fromEntries(mb).k.run();
}

function useFromEntriesMapSet() {
  const ma = new Map();
  ma.set("k", new A());
  const mb = new Map();
  mb.set("k", new B());
  return Object.fromEntries(ma).k.execute() + Object.fromEntries(mb).k.run();
}

function useFromEntriesMapBracket() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  return Object.fromEntries(ma)["k"].execute() + Object.fromEntries(mb)["k"].run();
}

function useFromEntriesMapAssign() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  const oa = Object.fromEntries(ma);
  const ob = Object.fromEntries(mb);
  return oa.k.execute() + ob.k.run();
}

function useFromEntriesMapValues() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  return (
    Object.values(Object.fromEntries(ma))[0].execute() +
    Object.values(Object.fromEntries(mb))[0].run()
  );
}

function useFromEntriesMapSpread() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  return (
    Object.fromEntries([...ma]).k.execute() + Object.fromEntries([...mb]).k.run()
  );
}

function useFromEntriesMapEntries() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  return (
    Object.fromEntries(ma.entries()).k.execute() +
    Object.fromEntries(mb.entries()).k.run()
  );
}

function useFromEntriesMapEntriesSpread() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  return (
    Object.fromEntries([...ma.entries()]).k.execute() +
    Object.fromEntries([...mb.entries()]).k.run()
  );
}

function useFromEntriesNewMap() {
  return (
    Object.fromEntries(new Map([["k", new A()]])).k.execute() +
    Object.fromEntries(new Map([["k", new B()]])).k.run()
  );
}

function usePreservesB() {
  const mb = new Map([["k", new B()]]);
  return (
    Object.fromEntries(mb).k.run() +
    Object.values(Object.fromEntries(mb))[0].run()
  );
}

function useFromEntriesArrayFrom() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  return (
    Object.fromEntries(Array.from(ma)).k.execute() +
    Object.fromEntries(Array.from(mb)).k.run()
  );
}
