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

function useObjectCreateProp() {
  return (
    Object.create({ k: new BoxA().get() }).k.run() +
    Object.create({ k: new BoxB().get() }).k.run()
  );
}

function useObjectCreatePropAssign() {
  const xa = Object.create({ k: new BoxA().get() }).k;
  const xb = Object.create({ k: new BoxB().get() }).k;
  return xa.run() + xb.run();
}

function useObjectCreatePropClass() {
  return (
    Object.create({ k: new A() }).k.run() +
    Object.create({ k: new B() }).k.run()
  );
}

function useIteratorFromNext() {
  return (
    Iterator.from([new BoxA().get()]).next().value.run() +
    Iterator.from([new BoxB().get()]).next().value.run()
  );
}

function useIteratorFromNextAssign() {
  const xa = Iterator.from([new BoxA().get()]).next().value;
  const xb = Iterator.from([new BoxB().get()]).next().value;
  return xa.run() + xb.run();
}

function useMapEntriesSpread() {
  return (
    [...new Map([["k", new BoxA().get()]]).entries()][0][1].run() +
    [...new Map([["k", new BoxB().get()]]).entries()][0][1].run()
  );
}

function useMapEntriesNext() {
  return (
    new Map([["k", new BoxA().get()]]).entries().next().value[1].run() +
    new Map([["k", new BoxB().get()]]).entries().next().value[1].run()
  );
}

function useSetKeysSpread() {
  return (
    [...new Set([new BoxA().get()]).keys()][0].run() +
    [...new Set([new BoxB().get()]).keys()][0].run()
  );
}

function useSetKeysNext() {
  return (
    new Set([new BoxA().get()]).keys().next().value.run() +
    new Set([new BoxB().get()]).keys().next().value.run()
  );
}

function useObjectSpreadProp() {
  return (
    { ...{ k: new BoxA().get() } }.k.run() +
    { ...{ k: new BoxB().get() } }.k.run()
  );
}

function useObjectSpreadAssign() {
  const xa = { ...{ k: new BoxA().get() } }.k;
  const xb = { ...{ k: new BoxB().get() } }.k;
  return xa.run() + xb.run();
}

function useFromEntriesEntries() {
  return (
    Object.fromEntries(Object.entries({ k: new BoxA().get() })).k.run() +
    Object.fromEntries(Object.entries({ k: new BoxB().get() })).k.run()
  );
}

function useFromEntriesEntriesLocal() {
  const oa = Object.fromEntries(Object.entries({ k: new BoxA().get() }));
  const ob = Object.fromEntries(Object.entries({ k: new BoxB().get() }));
  return oa.k.run() + ob.k.run();
}

function useFromEntriesMapEntries() {
  return (
    Object.fromEntries(new Map([["k", new BoxA().get()]]).entries()).k.run() +
    Object.fromEntries(new Map([["k", new BoxB().get()]]).entries()).k.run()
  );
}

function usePreservesB() {
  return (
    Object.create({ k: new BoxB().get() }).k.run() +
    Iterator.from([new BoxB().get()]).next().value.run() +
    [...new Map([["k", new BoxB().get()]]).entries()][0][1].run() +
    [...new Set([new BoxB().get()]).keys()][0].run() +
    { ...{ k: new BoxB().get() } }.k.run() +
    Object.fromEntries(Object.entries({ k: new BoxB().get() })).k.run()
  );
}
