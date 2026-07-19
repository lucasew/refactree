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

function useProxyProp() {
  return new Proxy({ k: new A() }, {}).k.run() + new Proxy({ k: new B() }, {}).k.run();
}

function useProxyPropLocal() {
  const oa = { k: new A() };
  const ob = { k: new B() };
  return new Proxy(oa, {}).k.run() + new Proxy(ob, {}).k.run();
}

function useProxyPropAssign() {
  const a = new Proxy({ k: new A() }, {}).k;
  const b = new Proxy({ k: new B() }, {}).k;
  return a.run() + b.run();
}

function useProxyPropBracket() {
  return new Proxy({ k: new A() }, {})["k"].run() + new Proxy({ k: new B() }, {})["k"].run();
}

function useProxyObjLocal() {
  const pa = new Proxy({ k: new A() }, {});
  const pb = new Proxy({ k: new B() }, {});
  return pa.k.run() + pb.k.run();
}

function useCreateProp() {
  return Object.create({ k: new A() }).k.run() + Object.create({ k: new B() }).k.run();
}

function useCreatePropLocal() {
  const oa = { k: new A() };
  const ob = { k: new B() };
  return Object.create(oa).k.run() + Object.create(ob).k.run();
}

function useFreezeProp() {
  return Object.freeze({ k: new A() }).k.run() + Object.freeze({ k: new B() }).k.run();
}

function useFreezePropLocal() {
  const oa = { k: new A() };
  const ob = { k: new B() };
  return Object.freeze(oa).k.run() + Object.freeze(ob).k.run();
}

function useSealProp() {
  return Object.seal({ k: new A() }).k.run() + Object.seal({ k: new B() }).k.run();
}

function usePreventExtProp() {
  return (
    Object.preventExtensions({ k: new A() }).k.run() +
    Object.preventExtensions({ k: new B() }).k.run()
  );
}
