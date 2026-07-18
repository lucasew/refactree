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

async function useFinallyAwait() {
  return (
    (await Promise.resolve(new A()).finally(() => {})).execute() +
    (await Promise.resolve(new B()).finally(() => {})).run()
  );
}

async function useFinallyLocal() {
  const a = await Promise.resolve(new A()).finally(() => {});
  const b = await Promise.resolve(new B()).finally(() => {});
  return a.execute() + b.run();
}

async function useFinallyThen() {
  return (
    (await Promise.resolve(new A()).finally(() => {}).then((x) => x)).execute() +
    (await Promise.resolve(new B()).finally(() => {}).then((x) => x)).run()
  );
}

async function useIdent() {
  const a = new A();
  const b = new B();
  return (
    (await Promise.resolve(a).finally(() => {})).execute() +
    (await Promise.resolve(b).finally(() => {})).run()
  );
}

async function usePreservesB() {
  return (
    (await Promise.resolve(new B()).finally(() => {})).run() +
    (await Promise.resolve(new B()).finally(() => {})).run()
  );
}
