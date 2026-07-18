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

function usePop() {
  return [new A()].pop().run() + [new B()].pop().run();
}

function useShift() {
  return [new A()].shift().run() + [new B()].shift().run();
}

function usePopLocal() {
  const a = [new A()].pop();
  const b = [new B()].pop();
  return a.run() + b.run();
}

function useShiftLocal() {
  const a = [new A()].shift();
  const b = [new B()].shift();
  return a.run() + b.run();
}

function usePopArrayLocal() {
  const as = [new A()];
  const bs = [new B()];
  return as.pop().run() + bs.pop().run();
}

function useShiftArrayLocal() {
  const as = [new A()];
  const bs = [new B()];
  return as.shift().run() + bs.shift().run();
}

function usePopAssign() {
  const as = [new A()];
  const bs = [new B()];
  const a = as.pop();
  const b = bs.pop();
  return a.run() + b.run();
}

function useArrayFromPop() {
  return (
    Array.from([new A()]).pop().run() + Array.from([new B()]).pop().run()
  );
}

function useSlicePop() {
  return [new A()].slice().pop().run() + [new B()].slice().pop().run();
}

function useIdent() {
  const a0 = new A();
  const b0 = new B();
  return [a0].pop().run() + [b0].shift().run();
}

function usePreservesB() {
  return (
    [new B()].pop().run() +
    [new B()].shift().run() +
    Array.from([new B()]).pop().run()
  );
}
