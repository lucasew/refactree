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

function useInline() {
  return (
    Iterator.from([new A()]).next().value.execute() +
    Iterator.from([new B()]).next().value.run()
  );
}

function useLocal() {
  const aa = [new A()];
  const bb = [new B()];
  return (
    Iterator.from(aa).next().value.execute() + Iterator.from(bb).next().value.run()
  );
}

function useAssign() {
  const a = Iterator.from([new A()]).next().value;
  const b = Iterator.from([new B()]).next().value;
  return a.execute() + b.run();
}

function usePreservesB() {
  return Iterator.from([new B()]).next().value.run();
}
