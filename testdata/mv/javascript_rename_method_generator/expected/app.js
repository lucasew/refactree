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

function* genA() {
  yield new A();
}

function* genB() {
  yield new B();
}

async function* agenA() {
  yield new A();
}

async function* agenB() {
  yield new B();
}

function useNextValue() {
  return genA().next().value.execute() + genB().next().value.run();
}

function useNextAssign() {
  const ra = genA().next();
  const rb = genB().next();
  return ra.value.execute() + rb.value.run();
}

function useNextValueAssign() {
  const a = genA().next().value;
  const b = genB().next().value;
  return a.execute() + b.run();
}

function useForOf() {
  let n = 0;
  for (const a of genA()) {
    n += a.execute();
  }
  for (const b of genB()) {
    n += b.run();
  }
  return n;
}

async function useForAwait() {
  let n = 0;
  for await (const a of agenA()) {
    n += a.execute();
  }
  for await (const b of agenB()) {
    n += b.run();
  }
  return n;
}

async function useAsyncNext() {
  return (
    (await agenA().next()).value.execute() +
    (await agenB().next()).value.run()
  );
}

async function useAsyncNextAssign() {
  const ra = await agenA().next();
  const rb = await agenB().next();
  return ra.value.execute() + rb.value.run();
}

function useIdentGen() {
  const ga = genA();
  const gb = genB();
  return ga.next().value.execute() + gb.next().value.run();
}

function usePreservesB() {
  return genB().next().value.run();
}
