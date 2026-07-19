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

function useIdent() {
  const a = new A();
  const b = new B();
  return (
    Promise.resolve(a).then((xa) => xa.execute()) +
    Promise.resolve(b).then((xb) => xb.run())
  );
}

function useIdentBare() {
  const a = new A();
  const b = new B();
  return (
    Promise.resolve(a).then(xa => xa.execute()) +
    Promise.resolve(b).then(xb => xb.run())
  );
}

async function useAwaitIdent() {
  const a = new A();
  const b = new B();
  const ra = await Promise.resolve(a);
  const rb = await Promise.resolve(b);
  return ra.execute() + rb.run();
}

async function useAwaitIdentChain() {
  const a = new A();
  const b = new B();
  return (
    (await Promise.resolve(a)).execute() +
    (await Promise.resolve(b)).run()
  );
}

function usePreservesB() {
  const b = new B();
  return Promise.resolve(b).then((xb) => xb.run());
}
