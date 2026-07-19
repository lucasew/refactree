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

function useMapFromPairsLocal() {
  const pa = [["k", new A()]];
  const pb = [["k", new B()]];
  return new Map(pa).get("k").execute() + new Map(pb).get("k").run();
}

function useMapFromPairsLocalAssign() {
  const pa = [["k", new A()]];
  const pb = [["k", new B()]];
  const ma = new Map(pa);
  const mb = new Map(pb);
  return ma.get("k").execute() + mb.get("k").run();
}

function useMapFromPairsLocalGetAssign() {
  const pa = [["k", new A()]];
  const pb = [["k", new B()]];
  const xa = new Map(pa).get("k");
  const xb = new Map(pb).get("k");
  return xa.execute() + xb.run();
}

function useMapFromPairsLocalEntries() {
  const pa = [["k", new A()]];
  const pb = [["k", new B()]];
  let n = 0;
  for (const [, xa] of new Map(pa).entries()) {
    n += xa.execute();
  }
  for (const [, xb] of new Map(pb).entries()) {
    n += xb.run();
  }
  return n;
}

function useMapFromPairsLocalValues() {
  const pa = [["k", new A()]];
  const pb = [["k", new B()]];
  return (
    new Map(pa).values().next().value.execute() +
    new Map(pb).values().next().value.run()
  );
}

function useMapFromObjectEntries() {
  return (
    new Map(Object.entries({ k: new A() })).get("k").execute() +
    new Map(Object.entries({ k: new B() })).get("k").run()
  );
}

function useMapFromObjectEntriesLocal() {
  const ea = Object.entries({ k: new A() });
  const eb = Object.entries({ k: new B() });
  return new Map(ea).get("k").execute() + new Map(eb).get("k").run();
}

function useMultiPair() {
  const pa = [
    ["k", new A()],
    ["j", new A()],
  ];
  const pb = [
    ["k", new B()],
    ["j", new B()],
  ];
  return new Map(pa).get("j").execute() + new Map(pb).get("j").run();
}

function useIdent() {
  const a = new A();
  const b = new B();
  const pa = [["k", a]];
  const pb = [["k", b]];
  return new Map(pa).get("k").execute() + new Map(pb).get("k").run();
}

function usePreservesB() {
  const pb = [["k", new B()]];
  return (
    new Map(pb).get("k").run() +
    new Map(Object.entries({ k: new B() })).get("k").run()
  );
}
