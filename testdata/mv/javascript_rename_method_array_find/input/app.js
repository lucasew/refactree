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

function useFind() {
  return (
    [new A()].find((x) => x).run() + [new B()].find((x) => x).run()
  );
}

function useFindLast() {
  return (
    [new A()].findLast((x) => x).run() + [new B()].findLast((x) => x).run()
  );
}

function useFindBare() {
  return [new A()].find(x => true).run() + [new B()].find(x => true).run();
}

function useFindLocal() {
  const a = [new A()].find((x) => x);
  const b = [new B()].find((x) => x);
  return a.run() + b.run();
}

function useFindLastLocal() {
  const a = [new A()].findLast((x) => x);
  const b = [new B()].findLast((x) => x);
  return a.run() + b.run();
}

function useFindArrayLocal() {
  const as = [new A()];
  const bs = [new B()];
  return as.find((x) => x).run() + bs.findLast((x) => x).run();
}

function useFindAssign() {
  const as = [new A()];
  const bs = [new B()];
  const a = as.find((x) => true);
  const b = bs.findLast((x) => true);
  return a.run() + b.run();
}

function useArrayFromFind() {
  return (
    Array.from([new A()]).find((x) => x).run() +
    Array.from([new B()]).find((x) => x).run()
  );
}

function useSliceFind() {
  return (
    [new A()].slice().find((x) => x).run() +
    [new B()].slice().findLast((x) => x).run()
  );
}

function useIdent() {
  const a0 = new A();
  const b0 = new B();
  return [a0].find((x) => x).run() + [b0].findLast((x) => x).run();
}

function usePreservesB() {
  return (
    [new B()].find((x) => x).run() +
    [new B()].findLast((x) => x).run() +
    Array.from([new B()]).find((x) => x).run()
  );
}
