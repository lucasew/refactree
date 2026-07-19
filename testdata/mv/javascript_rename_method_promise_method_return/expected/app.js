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

// Promise.resolve(method-return) under foreign same-leaf.
async function useResolveAssign() {
  const xa = await Promise.resolve(new BoxA().get());
  const xb = await Promise.resolve(new BoxB().get());
  return xa.execute() + xb.run();
}

async function useResolveInline() {
  return (
    (await Promise.resolve(new BoxA().get())).execute() +
    (await Promise.resolve(new BoxB().get())).run()
  );
}

function useResolveThen() {
  return (
    Promise.resolve(new BoxA().get()).then((a) => a.execute()) +
    Promise.resolve(new BoxB().get()).then((b) => b.run())
  );
}

function useResolveThenBare() {
  return (
    Promise.resolve(new BoxA().get()).then((a) => a.execute()) +
    Promise.resolve(new BoxB().get()).then((b) => b.run())
  );
}

// Promise.all / race / any / allSettled method-return.
async function useAllIndex() {
  return (
    (await Promise.all([new BoxA().get()]))[0].execute() +
    (await Promise.all([new BoxB().get()]))[0].run()
  );
}

async function useAllAssign() {
  const [xa] = await Promise.all([new BoxA().get()]);
  const [xb] = await Promise.all([new BoxB().get()]);
  return xa.execute() + xb.run();
}

async function useRaceAny() {
  return (
    (await Promise.race([new BoxA().get()])).execute() +
    (await Promise.race([new BoxB().get()])).run() +
    (await Promise.any([new BoxA().get()])).execute() +
    (await Promise.any([new BoxB().get()])).run()
  );
}

async function useSettledInline() {
  return (
    (await Promise.allSettled([new BoxA().get()]))[0].value.execute() +
    (await Promise.allSettled([new BoxB().get()]))[0].value.run()
  );
}

async function useSettledAssign() {
  const [ra] = await Promise.allSettled([new BoxA().get()]);
  const [rb] = await Promise.allSettled([new BoxB().get()]);
  return ra.value.execute() + rb.value.run();
}

// Class regression — already worked.
async function useClass() {
  return (
    (await Promise.resolve(new A())).execute() +
    (await Promise.resolve(new B())).run() +
    (await Promise.all([new A()]))[0].execute() +
    (await Promise.all([new B()]))[0].run() +
    (await Promise.race([new A()])).execute() +
    (await Promise.any([new B()])).run() +
    (await Promise.allSettled([new A()]))[0].value.execute() +
    (await Promise.allSettled([new B()]))[0].value.run() +
    Promise.resolve(new A()).then((a) => a.execute()) +
    Promise.resolve(new B()).then((b) => b.run())
  );
}

async function usePreservesB() {
  return (
    (await Promise.resolve(new BoxB().get())).run() +
    (await Promise.resolve(new B())).run() +
    (await Promise.all([new BoxB().get()]))[0].run() +
    (await Promise.all([new B()]))[0].run() +
    (await Promise.race([new BoxB().get()])).run() +
    (await Promise.any([new B()])).run() +
    (await Promise.allSettled([new BoxB().get()]))[0].value.run() +
    (await Promise.allSettled([new B()]))[0].value.run() +
    Promise.resolve(new BoxB().get()).then((b) => b.run()) +
    Promise.resolve(new B()).then((b) => b.run())
  );
}
