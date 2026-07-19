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

function useThenValue() {
  return (
    Promise.allSettled([new A()]).then(([ra]) => ra.value.run()) +
    Promise.allSettled([new B()]).then(([rb]) => rb.value.run())
  );
}

function useThenValueFn() {
  return (
    Promise.allSettled([new A()]).then(function ([ra]) {
      return ra.value.run();
    }) +
    Promise.allSettled([new B()]).then(function ([rb]) {
      return rb.value.run();
    })
  );
}

function useThenObjDestr() {
  return (
    Promise.allSettled([new A()]).then(([{ value: a }]) => a.run()) +
    Promise.allSettled([new B()]).then(([{ value: b }]) => b.run())
  );
}

function useThenObjShorthand() {
  return (
    Promise.allSettled([new A()]).then(([{ value: va }]) => va.run()) +
    Promise.allSettled([new B()]).then(([{ value: vb }]) => vb.run())
  );
}

async function useAwaitAssign() {
  const [ra] = await Promise.allSettled([new A()]);
  const [rb] = await Promise.allSettled([new B()]);
  return ra.value.run() + rb.value.run();
}

async function useAwaitObjDestr() {
  const [{ value: a }] = await Promise.allSettled([new A()]);
  const [{ value: b }] = await Promise.allSettled([new B()]);
  return a.run() + b.run();
}

async function useChain() {
  return (
    (await Promise.allSettled([new A()]))[0].value.run() +
    (await Promise.allSettled([new B()]))[0].value.run()
  );
}

async function useSubscriptAssign() {
  const ra = (await Promise.allSettled([new A()]))[0];
  const rb = (await Promise.allSettled([new B()]))[0];
  return ra.value.run() + rb.value.run();
}

function useIdent() {
  const a = new A();
  const b = new B();
  return (
    Promise.allSettled([a]).then(([ra]) => ra.value.run()) +
    Promise.allSettled([b]).then(([rb]) => rb.value.run())
  );
}

function usePreservesB() {
  return Promise.allSettled([new B()]).then(([rb]) => rb.value.run());
}
