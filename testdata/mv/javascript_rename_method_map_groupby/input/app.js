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

function useMapGroupByInline() {
  return (
    Map.groupBy([new A()], (x) => "k").get("k")[0].run() +
    Map.groupBy([new B()], (x) => "k").get("k")[0].run()
  );
}

function useMapGroupByLocal() {
  const ma = Map.groupBy([new A()], (x) => "k");
  const mb = Map.groupBy([new B()], (x) => "k");
  return ma.get("k")[0].run() + mb.get("k")[0].run();
}

function useMapGroupByForOf() {
  let n = 0;
  for (const xa of Map.groupBy([new A()], (x) => "k").get("k")) {
    n += xa.run();
  }
  for (const xb of Map.groupBy([new B()], (x) => "k").get("k")) {
    n += xb.run();
  }
  return n;
}

function useMapGroupByLocalForOf() {
  let n = 0;
  const ma = Map.groupBy([new A()], (x) => "k");
  const mb = Map.groupBy([new B()], (x) => "k");
  for (const xa of ma.get("k")) {
    n += xa.run();
  }
  for (const xb of mb.get("k")) {
    n += xb.run();
  }
  return n;
}

function useIdent() {
  const a0 = new A();
  const b0 = new B();
  return (
    Map.groupBy([a0], (x) => "k").get("k")[0].run() +
    Map.groupBy([b0], (x) => "k").get("k")[0].run()
  );
}

function usePreservesB() {
  return Map.groupBy([new B()], (x) => "k").get("k")[0].run();
}
