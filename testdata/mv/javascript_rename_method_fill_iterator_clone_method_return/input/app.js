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

function useFill() {
  return (
    Array(1).fill(new BoxA().get())[0].run() +
    Array(1).fill(new BoxB().get())[0].run()
  );
}

function useFillAssign() {
  const xs = Array(1).fill(new BoxA().get());
  const ys = Array(1).fill(new BoxB().get());
  return xs[0].run() + ys[0].run();
}

function useIteratorFrom() {
  return (
    Iterator.from([new BoxA().get()]).toArray()[0].run() +
    Iterator.from([new BoxB().get()]).toArray()[0].run()
  );
}

function useStructuredClone() {
  return (
    structuredClone(new BoxA().get()).run() +
    structuredClone(new BoxB().get()).run()
  );
}

function useSCAssign() {
  const xa = structuredClone(new BoxA().get());
  const xb = structuredClone(new BoxB().get());
  return xa.run() + xb.run();
}

function useWeakRef() {
  return (
    new WeakRef(new BoxA().get()).deref().run() +
    new WeakRef(new BoxB().get()).deref().run()
  );
}

function useWeakRefAssign() {
  const wa = new WeakRef(new BoxA().get());
  const wb = new WeakRef(new BoxB().get());
  return wa.deref().run() + wb.deref().run();
}

function useProxy() {
  return (
    new Proxy(new BoxA().get(), {}).run() +
    new Proxy(new BoxB().get(), {}).run()
  );
}

function useProxyAssign() {
  const pa = new Proxy(new BoxA().get(), {});
  const pb = new Proxy(new BoxB().get(), {});
  return pa.run() + pb.run();
}

function useMapValues() {
  return (
    [...new Map([["k", new BoxA().get()]]).values()][0].run() +
    [...new Map([["k", new BoxB().get()]]).values()][0].run()
  );
}

function useClass() {
  return (
    Array(1).fill(new A())[0].run() +
    Array(1).fill(new B())[0].run() +
    structuredClone(new A()).run() +
    structuredClone(new B()).run() +
    new WeakRef(new A()).deref().run() +
    new WeakRef(new B()).deref().run() +
    new Proxy(new A(), {}).run() +
    new Proxy(new B(), {}).run() +
    [...new Map([["k", new A()]]).values()][0].run() +
    [...new Map([["k", new B()]]).values()][0].run()
  );
}

function usePreservesB() {
  return (
    Array(1).fill(new BoxB().get())[0].run() +
    structuredClone(new BoxB().get()).run() +
    new WeakRef(new BoxB().get()).deref().run() +
    new Proxy(new BoxB().get(), {}).run() +
    [...new Map([["k", new BoxB().get()]]).values()][0].run()
  );
}
