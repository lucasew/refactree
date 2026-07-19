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

// Class regression — already solid.
function* genClassA() {
  yield new A();
}

function* genClassB() {
  yield new B();
}

// Method-return yields under foreign same-leaf.
function* genMRA() {
  yield new BoxA().get();
}

function* genMRB() {
  yield new BoxB().get();
}

function* genMRALocal() {
  const x = new BoxA().get();
  yield x;
}

function* genMRBLocal() {
  const x = new BoxB().get();
  yield x;
}

async function* agenMRA() {
  yield new BoxA().get();
}

async function* agenMRB() {
  yield new BoxB().get();
}

async function* agenClassA() {
  yield new A();
}

async function* agenClassB() {
  yield new B();
}

function useClassInline() {
  return genClassA().next().value.run() + genClassB().next().value.run();
}

function useMRDirectInline() {
  return genMRA().next().value.run() + genMRB().next().value.run();
}

function useMRLocalInline() {
  return genMRALocal().next().value.run() + genMRBLocal().next().value.run();
}

function useClassAssign() {
  const classXA = genClassA().next().value;
  const classXB = genClassB().next().value;
  return classXA.run() + classXB.run();
}

function useMRDirectAssign() {
  const mrXA = genMRA().next().value;
  const mrXB = genMRB().next().value;
  return mrXA.run() + mrXB.run();
}

function useMRLocalAssign() {
  const mrLA = genMRALocal().next().value;
  const mrLB = genMRBLocal().next().value;
  return mrLA.run() + mrLB.run();
}

function useClassFor() {
  let r = 0;
  for (const classXA of genClassA()) r += classXA.run();
  for (const classXB of genClassB()) r += classXB.run();
  return r;
}

function useMRDirectFor() {
  let r = 0;
  for (const mrXA of genMRA()) r += mrXA.run();
  for (const mrXB of genMRB()) r += mrXB.run();
  return r;
}

async function useAsyncClass() {
  return (
    (await agenClassA().next()).value.run() +
    (await agenClassB().next()).value.run()
  );
}

async function useAsyncMR() {
  return (
    (await agenMRA().next()).value.run() + (await agenMRB().next()).value.run()
  );
}

function usePreservesB() {
  return genMRB().next().value.run() + genClassB().next().value.run();
}
