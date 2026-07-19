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

function useMapNext() {
  return (
    Iterator.from([new A()]).map((x) => x).next().value.run() +
    Iterator.from([new B()]).map((x) => x).next().value.run()
  );
}

function useMapFilter() {
  return (
    Iterator.from([new A()])
      .map((x) => x)
      .filter(() => true)
      .next().value.run() +
    Iterator.from([new B()])
      .map((x) => x)
      .filter(() => true)
      .next().value.run()
  );
}

function useMapLocal() {
  const ia = Iterator.from([new A()]).map((x) => x);
  const ib = Iterator.from([new B()]).map((x) => x);
  return ia.next().value.run() + ib.next().value.run();
}

function usePreservesB() {
  return Iterator.from([new B()]).map((x) => x).next().value.run();
}
