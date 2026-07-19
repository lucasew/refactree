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

function useSlice() {
  return [new A()].slice()[0].run() + [new B()].slice()[0].run();
}

function useSliceArgs() {
  return [new A()].slice(0)[0].run() + [new B()].slice(0)[0].run();
}

function useSliceLocal() {
  const as = [new A()].slice();
  const bs = [new B()].slice();
  return as[0].run() + bs[0].run();
}

function useSliceAt() {
  return [new A()].slice().at(0).run() + [new B()].slice().at(0).run();
}

function useSliceForOf() {
  let n = 0;
  for (const xa of [new A()].slice()) {
    n += xa.run();
  }
  for (const xb of [new B()].slice()) {
    n += xb.run();
  }
  return n;
}

function useConcat() {
  return [new A()].concat()[0].run() + [new B()].concat()[0].run();
}

function useConcatArg() {
  return (
    [new A()].concat([new A()])[0].run() +
    [new B()].concat([new B()])[0].run()
  );
}

function useConcatElem() {
  return (
    [new A()].concat(new A())[0].run() + [new B()].concat(new B())[0].run()
  );
}

function useConcatLocal() {
  const as = [new A()].concat([new A()]);
  const bs = [new B()].concat([new B()]);
  return as[0].run() + bs[0].run();
}

function useToSpliced() {
  return (
    [new A()].toSpliced(0, 0)[0].run() + [new B()].toSpliced(0, 0)[0].run()
  );
}

function useToSplicedInsert() {
  return (
    [new A()].toSpliced(0, 0, new A())[0].run() +
    [new B()].toSpliced(0, 0, new B())[0].run()
  );
}

function useFlatIdentity() {
  return [new A()].flat()[0].run() + [new B()].flat()[0].run();
}

function useFlatNested() {
  return [[new A()]].flat()[0].run() + [[new B()]].flat()[0].run();
}

function useFlatDepth() {
  return [[new A()]].flat(1)[0].run() + [[new B()]].flat(1)[0].run();
}

function useFlatNestedLocal() {
  const ia = [new A()];
  const ib = [new B()];
  return [ia].flat()[0].run() + [ib].flat()[0].run();
}

function useFlatLocal() {
  const as = [[new A()]].flat();
  const bs = [[new B()]].flat();
  return as[0].run() + bs[0].run();
}

function useArrayFromSlice() {
  return (
    Array.from([new A()]).slice()[0].run() +
    Array.from([new B()]).slice()[0].run()
  );
}

function useIdent() {
  const a = new A();
  const b = new B();
  return [a].slice()[0].run() + [b].slice()[0].run();
}

function usePreservesB() {
  return (
    [new B()].slice()[0].run() +
    [new B()].concat()[0].run() +
    [new B()].toSpliced(0, 0)[0].run() +
    [[new B()]].flat()[0].run() +
    [new B()].flat()[0].run()
  );
}
