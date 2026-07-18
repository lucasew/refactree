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

function useRaceThen() {
  return (
    Promise.race([new A()]).then((a) => a.execute()) +
    Promise.race([new B()]).then((b) => b.run())
  );
}

function useAnyThen() {
  return (
    Promise.any([new A()]).then((a) => a.execute()) +
    Promise.any([new B()]).then((b) => b.run())
  );
}

async function useRaceAwait() {
  const a = await Promise.race([new A()]);
  const b = await Promise.race([new B()]);
  return a.execute() + b.run();
}

async function useRaceChain() {
  return (
    (await Promise.race([new A()])).execute() +
    (await Promise.race([new B()])).run()
  );
}

async function useAnyChain() {
  return (
    (await Promise.any([new A()])).execute() +
    (await Promise.any([new B()])).run()
  );
}

function useIdent() {
  const a = new A();
  const b = new B();
  return (
    Promise.race([a]).then((xa) => xa.execute()) +
    Promise.race([b]).then((xb) => xb.run())
  );
}

function usePreservesB() {
  return Promise.race([new B()]).then((b) => b.run());
}
