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

function useTryThen() {
  return (
    Promise.try(() => new A()).then((x) => x.run()) +
    Promise.try(() => new B()).then((y) => y.run())
  );
}

async function useTryAwait() {
  const a = await Promise.try(() => new A());
  const b = await Promise.try(() => new B());
  return a.run() + b.run();
}

function useTryLocal() {
  const a = new A();
  const b = new B();
  return (
    Promise.try(() => a).then((x) => x.run()) +
    Promise.try(() => b).then((y) => y.run())
  );
}

function usePreservesB() {
  return Promise.try(() => new B()).then((y) => y.run());
}
