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

function useFilterIndex() {
  return (
    [new A()].filter((x) => true)[0].execute() +
    [new B()].filter((x) => true)[0].run()
  );
}

function useFilterBare() {
  return (
    [new A()].filter(x => x)[0].execute() + [new B()].filter(x => x)[0].run()
  );
}

function useFilterLocal() {
  const as = [new A()].filter((x) => true);
  const bs = [new B()].filter((x) => true);
  return as[0].execute() + bs[0].run();
}

function useFilterForEach() {
  let n = 0;
  [new A()].filter((x) => true).forEach((va) => {
    n += va.execute();
  });
  [new B()].filter((x) => true).forEach((vb) => {
    n += vb.run();
  });
  return n;
}

function useFilterForEachLocal() {
  const as = [new A()];
  const bs = [new B()];
  let n = 0;
  as.filter((x) => true).forEach((va) => {
    n += va.execute();
  });
  bs.filter((x) => true).forEach((vb) => {
    n += vb.run();
  });
  return n;
}

function useFilterAt() {
  return (
    [new A()].filter((x) => true).at(0).execute() +
    [new B()].filter((x) => true).at(0).run()
  );
}

function useFilterSlice() {
  return (
    [new A()].slice().filter((x) => true)[0].execute() +
    [new B()].slice().filter((x) => true)[0].run()
  );
}

function useArrayFromFilter() {
  return (
    Array.from([new A()]).filter((x) => true)[0].execute() +
    Array.from([new B()]).filter((x) => true)[0].run()
  );
}

function useIdent() {
  const a = new A();
  const b = new B();
  return [a].filter((x) => true)[0].execute() + [b].filter((x) => true)[0].run();
}

function usePreservesB() {
  return (
    [new B()].filter((x) => true)[0].run() +
    [new B()].filter((x) => true).at(0).run() +
    Array.from([new B()]).filter((x) => true)[0].run()
  );
}
