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

function useReduceAcc() {
  return (
    [new A()].reduce((a, b) => a).run() + [new B()].reduce((a, b) => a).run()
  );
}

function useReduceCur() {
  return (
    [new A()].reduce((a, b) => b).run() + [new B()].reduce((a, b) => b).run()
  );
}

function useReduceBare() {
  return (
    [new A()].reduce((a, b) => a).run() + [new B()].reduce((a, b) => a).run()
  );
}

function useReduceBlock() {
  return (
    [new A()].reduce((a, b) => {
      return a;
    }).run() +
    [new B()].reduce((a, b) => {
      return a;
    }).run()
  );
}

function useReduceInit() {
  return (
    [new A()].reduce((a, b) => a, new A()).run() +
    [new B()].reduce((a, b) => a, new B()).run()
  );
}

function useReduceLocal() {
  const a = [new A()].reduce((x, y) => x);
  const b = [new B()].reduce((x, y) => x);
  return a.run() + b.run();
}

function useReduceArrayLocal() {
  const as = [new A()];
  const bs = [new B()];
  return as.reduce((a, b) => a).run() + bs.reduce((a, b) => a).run();
}

function useReduceSlice() {
  return (
    [new A()].slice().reduce((a, b) => a).run() +
    [new B()].slice().reduce((a, b) => a).run()
  );
}

function useArrayFromReduce() {
  return (
    Array.from([new A()]).reduce((a, b) => a).run() +
    Array.from([new B()]).reduce((a, b) => a).run()
  );
}

function useIdent() {
  const a0 = new A();
  const b0 = new B();
  return [a0].reduce((a, b) => a).run() + [b0].reduce((a, b) => a).run();
}


function useReduceRight() {
  return (
    [new A()].reduceRight((a, b) => a).run() +
    [new B()].reduceRight((a, b) => a).run()
  );
}

function useReduceRightCur() {
  return (
    [new A()].reduceRight((a, b) => b).run() +
    [new B()].reduceRight((a, b) => b).run()
  );
}

function usePreservesB() {
  return (
    [new B()].reduce((a, b) => a).run() +
    [new B()].reduce((a, b) => b).run() +
    [new B()].reduce((a, b) => a, new B()).run() +
    [new B()].reduceRight((a, b) => a).run()
  );
}
