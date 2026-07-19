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

function useLengthMap() {
  return (
    Array.from({ length: 1 }, () => new A())[0].execute() +
    Array.from({ length: 1 }, () => new B())[0].run()
  );
}

function useLengthMapIdx() {
  return (
    Array.from({ length: 1 }, (_v, i) => new A())[0].execute() +
    Array.from({ length: 1 }, (_v, i) => new B())[0].run()
  );
}

function useLengthMapBlock() {
  return (
    Array.from({ length: 1 }, () => {
      return new A();
    })[0].execute() +
    Array.from({ length: 1 }, () => {
      return new B();
    })[0].run()
  );
}

function useLengthMapLocal() {
  const as = Array.from({ length: 1 }, () => new A());
  const bs = Array.from({ length: 1 }, () => new B());
  return as[0].execute() + bs[0].run();
}

function useLengthMapForOf() {
  let n = 0;
  for (const a of Array.from({ length: 1 }, () => new A())) {
    n += a.execute();
  }
  for (const b of Array.from({ length: 1 }, () => new B())) {
    n += b.run();
  }
  return n;
}

function useLengthMapIdent() {
  const a0 = new A();
  const b0 = new B();
  return (
    Array.from({ length: 1 }, () => a0)[0].execute() +
    Array.from({ length: 1 }, () => b0)[0].run()
  );
}


async function useFromAsync() {
  return (
    (await Array.fromAsync({ length: 1 }, async () => new A()))[0].execute() +
    (await Array.fromAsync({ length: 1 }, async () => new B()))[0].run()
  );
}

function usePreservesB() {
  return Array.from({ length: 1 }, () => new B())[0].run();
}
