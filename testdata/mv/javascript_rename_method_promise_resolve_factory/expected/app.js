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

function makeA() {
  return new A();
}

function makeB() {
  return new B();
}

const makeAArrow = () => new A();
const makeBArrow = () => new B();

const makeABlock = () => {
  return new A();
};

function useThen() {
  return (
    Promise.resolve(makeA()).then((a) => a.execute()) +
    Promise.resolve(makeB()).then((b) => b.run())
  );
}

function useThenBare() {
  return (
    Promise.resolve(makeA()).then(a => a.execute()) +
    Promise.resolve(makeB()).then(b => b.run())
  );
}

function useArrowFactory() {
  return (
    Promise.resolve(makeAArrow()).then((a) => a.execute()) +
    Promise.resolve(makeBArrow()).then((b) => b.run())
  );
}

function useBlockFactory() {
  return Promise.resolve(makeABlock()).then((a) => a.execute());
}

async function useAwaitAssign() {
  const a = await Promise.resolve(makeA());
  const b = await Promise.resolve(makeB());
  return a.execute() + b.run();
}

async function useAwaitChain() {
  return (
    (await Promise.resolve(makeA())).execute() +
    (await Promise.resolve(makeB())).run()
  );
}

function usePreservesB() {
  return Promise.resolve(makeB()).then((b) => b.run());
}
