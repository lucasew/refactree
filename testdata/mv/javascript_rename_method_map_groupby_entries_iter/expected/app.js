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

function useForOfDestructure() {
  let n = 0;
  const ma = Map.groupBy([new A()], (x) => "k");
  const mb = Map.groupBy([new B()], (x) => "k");
  for (const [, ga] of ma.entries()) {
    n += ga[0].execute();
  }
  for (const [, gb] of mb.entries()) {
    n += gb[0].run();
  }
  return n;
}

function useForOfMap() {
  let n = 0;
  const ma = Map.groupBy([new A()], (x) => "k");
  const mb = Map.groupBy([new B()], (x) => "k");
  for (const [, ga] of ma) {
    n += ga[0].execute();
  }
  for (const [, gb] of mb) {
    n += gb[0].run();
  }
  return n;
}

function useForOfPair() {
  let n = 0;
  const ma = Map.groupBy([new A()], (x) => "k");
  const mb = Map.groupBy([new B()], (x) => "k");
  for (const ea of ma.entries()) {
    n += ea[1][0].execute();
  }
  for (const eb of mb.entries()) {
    n += eb[1][0].run();
  }
  return n;
}

function useNextValue() {
  return (
    Map.groupBy([new A()], (x) => "k").entries().next().value[1][0].execute() +
    Map.groupBy([new B()], (x) => "k").entries().next().value[1][0].run()
  );
}

function useNextLocal() {
  const ma = Map.groupBy([new A()], (x) => "k");
  const mb = Map.groupBy([new B()], (x) => "k");
  return ma.entries().next().value[1][0].execute() + mb.entries().next().value[1][0].run();
}

function useNextPairLocal() {
  const ea = Map.groupBy([new A()], (x) => "k").entries().next().value;
  const eb = Map.groupBy([new B()], (x) => "k").entries().next().value;
  return ea[1][0].execute() + eb[1][0].run();
}

function useValuesForOf() {
  let n = 0;
  const ma = Map.groupBy([new A()], (x) => "k");
  const mb = Map.groupBy([new B()], (x) => "k");
  for (const ga of ma.values()) {
    n += ga[0].execute();
  }
  for (const gb of mb.values()) {
    n += gb[0].run();
  }
  return n;
}

function usePreservesB() {
  let n = 0;
  const mb = Map.groupBy([new B()], (x) => "k");
  for (const [, g] of mb.entries()) {
    n += g[0].run();
  }
  n += mb.entries().next().value[1][0].run();
  return n;
}
