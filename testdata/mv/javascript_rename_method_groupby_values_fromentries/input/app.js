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

function useFromEntriesMapInline() {
  return (
    Object.fromEntries(Map.groupBy([new A()], (x) => "k"))["k"][0].run() +
    Object.fromEntries(Map.groupBy([new B()], (x) => "k"))["k"][0].run()
  );
}

function useFromEntriesMapLocal() {
  const ma = Map.groupBy([new A()], (x) => "k");
  const mb = Map.groupBy([new B()], (x) => "k");
  const oa = Object.fromEntries(ma);
  const ob = Object.fromEntries(mb);
  return oa["k"][0].run() + ob["k"][0].run();
}

function useFromEntriesMapDot() {
  const ma = Map.groupBy([new A()], (x) => "k");
  const mb = Map.groupBy([new B()], (x) => "k");
  return Object.fromEntries(ma).k[0].run() + Object.fromEntries(mb).k[0].run();
}

function useObjectValuesLocal() {
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  return Object.values(ga)[0][0].run() + Object.values(gb)[0][0].run();
}

function useObjectValuesForOf() {
  let n = 0;
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  for (const gaArr of Object.values(ga)) {
    n += gaArr[0].run();
  }
  for (const gbArr of Object.values(gb)) {
    n += gbArr[0].run();
  }
  return n;
}

function useObjectValuesForOfInline() {
  let n = 0;
  for (const gaArr of Object.values(Object.groupBy([new A()], (x) => "k"))) {
    n += gaArr[0].run();
  }
  for (const gbArr of Object.values(Object.groupBy([new B()], (x) => "k"))) {
    n += gbArr[0].run();
  }
  return n;
}

function useMapValuesNext() {
  const ma = Map.groupBy([new A()], (x) => "k");
  const mb = Map.groupBy([new B()], (x) => "k");
  return ma.values().next().value[0].run() + mb.values().next().value[0].run();
}

function useMapValuesNextInline() {
  return (
    Map.groupBy([new A()], (x) => "k").values().next().value[0].run() +
    Map.groupBy([new B()], (x) => "k").values().next().value[0].run()
  );
}

function useDestructureGet() {
  const ma = Map.groupBy([new A()], (x) => "k");
  const mb = Map.groupBy([new B()], (x) => "k");
  const [a0] = ma.get("k");
  const [b0] = mb.get("k");
  return a0.run() + b0.run();
}

function useDestructureGetInline() {
  const [a0] = Map.groupBy([new A()], (x) => "k").get("k");
  const [b0] = Map.groupBy([new B()], (x) => "k").get("k");
  return a0.run() + b0.run();
}

function usePreservesB() {
  const mb = Map.groupBy([new B()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  const [b0] = mb.get("k");
  return (
    Object.fromEntries(mb)["k"][0].run() +
    Object.values(gb)[0][0].run() +
    mb.values().next().value[0].run() +
    b0.run()
  );
}
