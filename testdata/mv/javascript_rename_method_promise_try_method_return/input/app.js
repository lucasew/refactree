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

class BoxA {
  a = new A();
  get() {
    return this.a;
  }
}

class BoxB {
  b = new B();
  get() {
    return this.b;
  }
}

// Promise.try(method-return) under foreign same-leaf.
async function useTryAwait() {
  return (
    (await Promise.try(() => new BoxA().get())).run() +
    (await Promise.try(() => new BoxB().get())).run()
  );
}

async function useTryAssign() {
  const xa = await Promise.try(() => new BoxA().get());
  const xb = await Promise.try(() => new BoxB().get());
  return xa.run() + xb.run();
}

function useTryThen() {
  return (
    Promise.try(() => new BoxA().get()).then((a) => a.run()) +
    Promise.try(() => new BoxB().get()).then((b) => b.run())
  );
}

// Class regression — already worked.
async function useClass() {
  return (
    (await Promise.try(() => new A())).run() +
    (await Promise.try(() => new B())).run() +
    Promise.try(() => new A()).then((a) => a.run()) +
    Promise.try(() => new B()).then((b) => b.run())
  );
}

async function usePreservesB() {
  return (
    (await Promise.try(() => new BoxB().get())).run() +
    (await Promise.try(() => new B())).run() +
    Promise.try(() => new BoxB().get()).then((b) => b.run()) +
    Promise.try(() => new B()).then((b) => b.run())
  );
}
