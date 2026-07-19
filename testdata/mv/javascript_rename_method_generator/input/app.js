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
  return genA().next().value.run() + genB().next().value.run();
}

function useNextAssign() {
  const ra = genA().next();
  const rb = genB().next();
  return ra.value.run() + rb.value.run();
}

function useNextValueAssign() {
  const a = genA().next().value;
  const b = genB().next().value;
  return a.run() + b.run();
}

function useForOf() {
  let n = 0;
  for (const a of genA()) {
    n += a.run();
  }
  for (const b of genB()) {
    n += b.run();
  }
  return n;
}

async function useForAwait() {
  let n = 0;
  for await (const a of agenA()) {
    n += a.run();
  }
  for await (const b of agenB()) {
    n += b.run();
  }
  return n;
}

async function useAsyncNext() {
  return (
    (await agenA().next()).value.run() +
    (await agenB().next()).value.run()
  );
}

async function useAsyncNextAssign() {
  const ra = await agenA().next();
  const rb = await agenB().next();
  return ra.value.run() + rb.value.run();
}

function useIdentGen() {
  const ga = genA();
  const gb = genB();
  return ga.next().value.run() + gb.next().value.run();
}

function usePreservesB() {
  return genB().next().value.run();
}
