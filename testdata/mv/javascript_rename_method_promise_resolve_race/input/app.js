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

async function useRaceResolve() {
  return (
    (await Promise.race([Promise.resolve(new A())])).run() +
    (await Promise.race([Promise.resolve(new B())])).run()
  );
}

async function useAnyResolve() {
  return (
    (await Promise.any([Promise.resolve(new A())])).run() +
    (await Promise.any([Promise.resolve(new B())])).run()
  );
}

async function useRaceResolveLocal() {
  const xa = await Promise.race([Promise.resolve(new A())]);
  const xb = await Promise.race([Promise.resolve(new B())]);
  return xa.run() + xb.run();
}

async function useRaceResolveIdent() {
  const a0 = new A();
  const b0 = new B();
  return (
    (await Promise.race([Promise.resolve(a0)])).run() +
    (await Promise.race([Promise.resolve(b0)])).run()
  );
}

async function usePreservesB() {
  return (await Promise.race([Promise.resolve(new B())])).run();
}
