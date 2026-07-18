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

async function useCatchAwait() {
  return (
    (await Promise.resolve(new A()).catch(() => null)).run() +
    (await Promise.resolve(new B()).catch(() => null)).run()
  );
}

async function useCatchLocal() {
  const xa = await Promise.resolve(new A()).catch(() => null);
  const xb = await Promise.resolve(new B()).catch(() => null);
  return xa.run() + xb.run();
}

async function useCatchThen() {
  return (
    Promise.resolve(new A())
      .catch(() => null)
      .then((a) => a.run()) +
    Promise.resolve(new B())
      .catch(() => null)
      .then((b) => b.run())
  );
}

async function useCatchThenIdentity() {
  return (
    (await Promise.resolve(new A()).catch(() => null).then((x) => x)).run() +
    (await Promise.resolve(new B()).catch(() => null).then((x) => x)).run()
  );
}

async function useCatchFinally() {
  return (
    (await Promise.resolve(new A()).catch(() => null).finally(() => {})).run() +
    (await Promise.resolve(new B()).catch(() => null).finally(() => {})).run()
  );
}

async function useIdent() {
  const a0 = new A();
  const b0 = new B();
  return (
    (await Promise.resolve(a0).catch(() => null)).run() +
    (await Promise.resolve(b0).catch(() => null)).run()
  );
}

async function usePreservesB() {
  return (await Promise.resolve(new B()).catch(() => null)).run();
}
