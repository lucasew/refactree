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

function usePop() {
  return [new A()].pop().execute() + [new B()].pop().run();
}

function useShift() {
  return [new A()].shift().execute() + [new B()].shift().run();
}

function usePopLocal() {
  const a = [new A()].pop();
  const b = [new B()].pop();
  return a.execute() + b.run();
}

function useShiftLocal() {
  const a = [new A()].shift();
  const b = [new B()].shift();
  return a.execute() + b.run();
}

function usePopArrayLocal() {
  const as = [new A()];
  const bs = [new B()];
  return as.pop().execute() + bs.pop().run();
}

function useShiftArrayLocal() {
  const as = [new A()];
  const bs = [new B()];
  return as.shift().execute() + bs.shift().run();
}

function usePopAssign() {
  const as = [new A()];
  const bs = [new B()];
  const a = as.pop();
  const b = bs.pop();
  return a.execute() + b.run();
}

function useArrayFromPop() {
  return (
    Array.from([new A()]).pop().execute() + Array.from([new B()]).pop().run()
  );
}

function useSlicePop() {
  return [new A()].slice().pop().execute() + [new B()].slice().pop().run();
}

function useIdent() {
  const a0 = new A();
  const b0 = new B();
  return [a0].pop().execute() + [b0].shift().run();
}

function usePreservesB() {
  return (
    [new B()].pop().run() +
    [new B()].shift().run() +
    Array.from([new B()]).pop().run()
  );
}
