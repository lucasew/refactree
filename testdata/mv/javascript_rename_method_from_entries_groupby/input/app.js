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

function useFromEntriesInline() {
  return (
    Object.fromEntries(Object.entries(Object.groupBy([new A()], (x) => "k")))["k"][0].run() +
    Object.fromEntries(Object.entries(Object.groupBy([new B()], (x) => "k")))["k"][0].run()
  );
}

function useFromEntriesLocal() {
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  return (
    Object.fromEntries(Object.entries(ga))["k"][0].run() +
    Object.fromEntries(Object.entries(gb))["k"][0].run()
  );
}

function useFromEntriesResultLocal() {
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  const oa = Object.fromEntries(Object.entries(ga));
  const ob = Object.fromEntries(Object.entries(gb));
  return oa["k"][0].run() + ob["k"][0].run();
}

function useFromEntriesDot() {
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  return Object.fromEntries(Object.entries(ga)).k[0].run() + Object.fromEntries(Object.entries(gb)).k[0].run();
}

function useFromEntriesSpread() {
  const ga = Object.groupBy([new A()], (x) => "k");
  const gb = Object.groupBy([new B()], (x) => "k");
  return (
    Object.fromEntries([...Object.entries(ga)])["k"][0].run() +
    Object.fromEntries([...Object.entries(gb)])["k"][0].run()
  );
}

function usePreservesB() {
  const gb = Object.groupBy([new B()], (x) => "k");
  return Object.fromEntries(Object.entries(gb))["k"][0].run();
}
