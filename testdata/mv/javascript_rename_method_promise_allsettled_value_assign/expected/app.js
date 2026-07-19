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

async function useValueAssign() {
  const [ra] = await Promise.allSettled([new A()]);
  const [rb] = await Promise.allSettled([new B()]);
  const a = ra.value;
  const b = rb.value;
  return a.execute() + b.run();
}

async function useValueAssignInline() {
  const ra = (await Promise.allSettled([new A()]))[0];
  const rb = (await Promise.allSettled([new B()]))[0];
  const a = ra.value;
  const b = rb.value;
  return a.execute() + b.run();
}

async function useValueDirect() {
  const [ra] = await Promise.allSettled([new A()]);
  const [rb] = await Promise.allSettled([new B()]);
  return ra.value.execute() + rb.value.run();
}
