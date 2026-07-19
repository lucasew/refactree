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

function useThenIdentityThen() {
  return (
    Promise.resolve(new A())
      .then((x) => x)
      .then((a) => a.run()) +
    Promise.resolve(new B())
      .then((x) => x)
      .then((b) => b.run())
  );
}

function useThenIdentityBare() {
  return (
    Promise.resolve(new A())
      .then(x => x)
      .then(a => a.run()) +
    Promise.resolve(new B())
      .then(x => x)
      .then(b => b.run())
  );
}

function useThenIdentityBlock() {
  return (
    Promise.resolve(new A())
      .then((x) => {
        return x;
      })
      .then((a) => a.run()) +
    Promise.resolve(new B())
      .then((x) => {
        return x;
      })
      .then((b) => b.run())
  );
}

async function useAwaitThenIdentity() {
  return (
    (await Promise.resolve(new A()).then((x) => x)).run() +
    (await Promise.resolve(new B()).then((x) => x)).run()
  );
}

async function useAwaitThenIdentityLocal() {
  const a = await Promise.resolve(new A()).then((x) => x);
  const b = await Promise.resolve(new B()).then((x) => x);
  return a.run() + b.run();
}

async function useDoubleIdentity() {
  return (
    (await Promise.resolve(new A()).then((x) => x).then((y) => y)).run() +
    (await Promise.resolve(new B()).then((x) => x).then((y) => y)).run()
  );
}

function useIdent() {
  const a0 = new A();
  const b0 = new B();
  return (
    Promise.resolve(a0)
      .then((x) => x)
      .then((a) => a.run()) +
    Promise.resolve(b0)
      .then((x) => x)
      .then((b) => b.run())
  );
}

function usePreservesB() {
  return (
    Promise.resolve(new B())
      .then((x) => x)
      .then((b) => b.run())
  );
}
