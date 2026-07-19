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

function useArrayEntriesNextValuePair() {
  return (
    [new A()].entries().next().value[1].execute() +
    [new B()].entries().next().value[1].run()
  );
}

function useArrayEntriesNextValueLocal() {
  const ea = [new A()].entries().next().value;
  const eb = [new B()].entries().next().value;
  return ea[1].execute() + eb[1].run();
}

function useArrayEntriesNextLocal() {
  const ra = [new A()].entries().next();
  const rb = [new B()].entries().next();
  return ra.value[1].execute() + rb.value[1].run();
}

function useArrayEntriesNextValueDestructure() {
  const [, xa] = [new A()].entries().next().value;
  const [, xb] = [new B()].entries().next().value;
  return xa.execute() + xb.run();
}

function useArrayEntriesLocalNext() {
  const ia = [new A()].entries();
  const ib = [new B()].entries();
  return ia.next().value[1].execute() + ib.next().value[1].run();
}

function useArrayEntriesNextValueAssign() {
  const xa = [new A()].entries().next().value[1];
  const xb = [new B()].entries().next().value[1];
  return xa.execute() + xb.run();
}

function useArrayEntriesLocalNextAssign() {
  const ia = [new A()].entries();
  const ib = [new B()].entries();
  const ra = ia.next();
  const rb = ib.next();
  return ra.value[1].execute() + rb.value[1].run();
}

function useArrayEntriesLocalNextPair() {
  const ia = [new A()].entries();
  const ib = [new B()].entries();
  const ea = ia.next().value;
  const eb = ib.next().value;
  return ea[1].execute() + eb[1].run();
}

function useIdent() {
  const a = new A();
  const b = new B();
  return (
    [a].entries().next().value[1].execute() + [b].entries().next().value[1].run()
  );
}

function useArrayLocal() {
  const as = [new A()];
  const bs = [new B()];
  return as.entries().next().value[1].execute() + bs.entries().next().value[1].run();
}

function usePreservesB() {
  const eb = [new B()].entries().next().value;
  const rb = [new B()].entries().next();
  return (
    [new B()].entries().next().value[1].run() +
    eb[1].run() +
    rb.value[1].run()
  );
}
