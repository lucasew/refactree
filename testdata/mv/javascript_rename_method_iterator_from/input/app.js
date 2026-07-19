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

function useInline() {
  return (
    Iterator.from([new A()]).next().value.run() +
    Iterator.from([new B()]).next().value.run()
  );
}

function useLocal() {
  const aa = [new A()];
  const bb = [new B()];
  return (
    Iterator.from(aa).next().value.run() + Iterator.from(bb).next().value.run()
  );
}

function useAssign() {
  const a = Iterator.from([new A()]).next().value;
  const b = Iterator.from([new B()]).next().value;
  return a.run() + b.run();
}

function usePreservesB() {
  return Iterator.from([new B()]).next().value.run();
}
