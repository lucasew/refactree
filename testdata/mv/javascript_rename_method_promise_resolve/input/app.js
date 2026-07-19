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

function useThen() {
  return (
    Promise.resolve(new A()).then((a) => a.run()) +
    Promise.resolve(new B()).then((b) => b.run())
  );
}

function useThenBare() {
  return (
    Promise.resolve(new A()).then(a => a.run()) +
    Promise.resolve(new B()).then(b => b.run())
  );
}

async function useAwaitAssign() {
  const a = await Promise.resolve(new A());
  const b = await Promise.resolve(new B());
  return a.run() + b.run();
}

async function useAwaitChain() {
  return (
    (await Promise.resolve(new A())).run() +
    (await Promise.resolve(new B())).run()
  );
}

async function usePreservesB() {
  const b = await Promise.resolve(new B());
  return b.run();
}
