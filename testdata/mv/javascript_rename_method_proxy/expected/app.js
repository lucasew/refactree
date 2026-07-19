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

function useProxy() {
  const a = new A();
  const b = new B();
  const pa = new Proxy(a, {});
  const pb = new Proxy(b, {});
  return pa.execute() + pb.run();
}

function useProxyInline() {
  const a = new A();
  const b = new B();
  return new Proxy(a, {}).execute() + new Proxy(b, {}).run();
}

function useProxyCtor() {
  return new Proxy(new A(), {}).execute() + new Proxy(new B(), {}).run();
}
