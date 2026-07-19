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
    Promise.resolve(makeA()).then((a) => a.run()) +
    Promise.resolve(makeB()).then((b) => b.run())
  );
}

function useThenBare() {
  return (
    Promise.resolve(makeA()).then(a => a.run()) +
    Promise.resolve(makeB()).then(b => b.run())
  );
}

function useArrowFactory() {
  return (
    Promise.resolve(makeAArrow()).then((a) => a.run()) +
    Promise.resolve(makeBArrow()).then((b) => b.run())
  );
}

function useBlockFactory() {
  return Promise.resolve(makeABlock()).then((a) => a.run());
}

async function useAwaitAssign() {
  const a = await Promise.resolve(makeA());
  const b = await Promise.resolve(makeB());
  return a.run() + b.run();
}

async function useAwaitChain() {
  return (
    (await Promise.resolve(makeA())).run() +
    (await Promise.resolve(makeB())).run()
  );
}

function usePreservesB() {
  return Promise.resolve(makeB()).then((b) => b.run());
}
