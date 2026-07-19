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

// --- Class regressions: then/finally/catch identity (already solid). ---
async function useClassAwaitThen() {
  return (
    (await Promise.resolve(new A()).then((x) => x)).execute() +
    (await Promise.resolve(new B()).then((x) => x)).run()
  );
}

async function useClassAwaitThenLocal() {
  const cxa = await Promise.resolve(new A()).then((x) => x);
  const cxb = await Promise.resolve(new B()).then((x) => x);
  return cxa.execute() + cxb.run();
}

async function useClassFinally() {
  return (
    (await Promise.resolve(new A()).finally(() => {})).execute() +
    (await Promise.resolve(new B()).finally(() => {})).run()
  );
}

async function useClassFinallyLocal() {
  const cfa = await Promise.resolve(new A()).finally(() => {});
  const cfb = await Promise.resolve(new B()).finally(() => {});
  return cfa.execute() + cfb.run();
}

async function useClassCatch() {
  return (
    (await Promise.resolve(new A()).catch(() => null)).execute() +
    (await Promise.resolve(new B()).catch(() => null)).run()
  );
}

async function useClassCatchLocal() {
  const cca = await Promise.resolve(new A()).catch(() => null);
  const ccb = await Promise.resolve(new B()).catch(() => null);
  return cca.execute() + ccb.run();
}

async function useClassDoubleThen() {
  return (
    (await Promise.resolve(new A()).then((x) => x).then((y) => y)).execute() +
    (await Promise.resolve(new B()).then((x) => x).then((y) => y)).run()
  );
}

// --- Method-return under foreign same-leaf. ---
async function useMrAwaitThen() {
  return (
    (await Promise.resolve(new BoxA().get()).then((x) => x)).execute() +
    (await Promise.resolve(new BoxB().get()).then((x) => x)).run()
  );
}

async function useMrAwaitThenLocal() {
  const mxa = await Promise.resolve(new BoxA().get()).then((x) => x);
  const mxb = await Promise.resolve(new BoxB().get()).then((x) => x);
  return mxa.execute() + mxb.run();
}

async function useMrFinally() {
  return (
    (await Promise.resolve(new BoxA().get()).finally(() => {})).execute() +
    (await Promise.resolve(new BoxB().get()).finally(() => {})).run()
  );
}

async function useMrFinallyLocal() {
  const mfa = await Promise.resolve(new BoxA().get()).finally(() => {});
  const mfb = await Promise.resolve(new BoxB().get()).finally(() => {});
  return mfa.execute() + mfb.run();
}

async function useMrCatch() {
  return (
    (await Promise.resolve(new BoxA().get()).catch(() => null)).execute() +
    (await Promise.resolve(new BoxB().get()).catch(() => null)).run()
  );
}

async function useMrCatchLocal() {
  const mca = await Promise.resolve(new BoxA().get()).catch(() => null);
  const mcb = await Promise.resolve(new BoxB().get()).catch(() => null);
  return mca.execute() + mcb.run();
}

async function useMrDoubleThen() {
  return (
    (await Promise.resolve(new BoxA().get()).then((x) => x).then((y) => y)).execute() +
    (await Promise.resolve(new BoxB().get()).then((x) => x).then((y) => y)).run()
  );
}

async function usePreservesB() {
  return (
    (await Promise.resolve(new BoxB().get()).then((x) => x)).run() +
    (await Promise.resolve(new BoxB().get()).finally(() => {})).run() +
    (await Promise.resolve(new BoxB().get()).catch(() => null)).run() +
    (await Promise.resolve(new B()).then((x) => x)).run()
  );
}
