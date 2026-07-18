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

function useThen() {
  return (
    Promise.all([new A()]).then(([a]) => a.execute()) +
    Promise.all([new B()]).then(([b]) => b.run())
  );
}

function useThenBare() {
  return (
    Promise.all([new A()]).then(([a]) => a.execute()) +
    Promise.all([new B()]).then(([b]) => b.run())
  );
}

function useThenFn() {
  return (
    Promise.all([new A()]).then(function ([a]) {
      return a.execute();
    }) +
    Promise.all([new B()]).then(function ([b]) {
      return b.run();
    })
  );
}

async function useAwaitAssign() {
  const [a] = await Promise.all([new A()]);
  const [b] = await Promise.all([new B()]);
  return a.execute() + b.run();
}

async function useAwaitIndex() {
  return (
    (await Promise.all([new A()]))[0].execute() +
    (await Promise.all([new B()]))[0].run()
  );
}

function useIdent() {
  const a = new A();
  const b = new B();
  return (
    Promise.all([a]).then(([xa]) => xa.execute()) +
    Promise.all([b]).then(([xb]) => xb.run())
  );
}

function usePreservesB() {
  return Promise.all([new B()]).then(([b]) => b.run());
}
