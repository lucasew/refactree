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

function useGroupByKey() {
  return (
    Object.groupBy([new A()], (x) => "k")["k"][0].execute() +
    Object.groupBy([new B()], (x) => "k")["k"][0].run()
  );
}

function useGroupByValues() {
  return (
    Object.values(Object.groupBy([new A()], (x) => "k"))[0][0].execute() +
    Object.values(Object.groupBy([new B()], (x) => "k"))[0][0].run()
  );
}

function useGroupByForOfKey() {
  let n = 0;
  for (const xa of Object.groupBy([new A()], (x) => "k")["k"]) {
    n += xa.execute();
  }
  for (const xb of Object.groupBy([new B()], (x) => "k")["k"]) {
    n += xb.run();
  }
  return n;
}

function useGroupByLocalGroup() {
  const ga = Object.values(Object.groupBy([new A()], (x) => "k"))[0];
  const gb = Object.values(Object.groupBy([new B()], (x) => "k"))[0];
  return ga[0].execute() + gb[0].run();
}

function useIdent() {
  const a0 = new A();
  const b0 = new B();
  return (
    Object.groupBy([a0], (x) => "k")["k"][0].execute() +
    Object.groupBy([b0], (x) => "k")["k"][0].run()
  );
}

function usePreservesB() {
  return Object.groupBy([new B()], (x) => "k")["k"][0].run();
}
