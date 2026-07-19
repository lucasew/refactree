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

function useFlatMap() {
  return (
    [0].flatMap(() => [new A()])[0].execute() +
    [0].flatMap(() => [new B()])[0].run()
  );
}

function useFlatMapLocal() {
  const as = [0].flatMap(() => [new A()]);
  const bs = [0].flatMap(() => [new B()]);
  return as[0].execute() + bs[0].run();
}

function useFlatMapForOf() {
  let n = 0;
  for (const a of [0].flatMap(() => [new A()])) {
    n += a.execute();
  }
  for (const b of [0].flatMap(() => [new B()])) {
    n += b.run();
  }
  return n;
}

function useFlatMapIdent() {
  const a0 = new A();
  const b0 = new B();
  return (
    [0].flatMap(() => [a0])[0].execute() + [0].flatMap(() => [b0])[0].run()
  );
}

function usePreservesB() {
  return [0].flatMap(() => [new B()])[0].run();
}
