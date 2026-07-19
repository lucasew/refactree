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

// Iterator.flatMap sole-return array peels yield T for next().value / local / for-of.
function useFlatMapNext() {
  return (
    Iterator.from([0]).flatMap(() => [new A()]).next().value.execute() +
    Iterator.from([0]).flatMap(() => [new B()]).next().value.run()
  );
}

function useFlatMapLocal() {
  const ia = Iterator.from([0]).flatMap(() => [new A()]);
  const ib = Iterator.from([0]).flatMap(() => [new B()]);
  return ia.next().value.execute() + ib.next().value.run();
}

function useFlatMapForOf() {
  let s = 0;
  for (const x of Iterator.from([0]).flatMap(() => [new A()])) s += x.execute();
  for (const y of Iterator.from([0]).flatMap(() => [new B()])) s += y.run();
  return s;
}

// Iterator.find / reduce identity peels.
function useFind() {
  return (
    Iterator.from([new A()]).find((x) => true).execute() +
    Iterator.from([new B()]).find((y) => true).run()
  );
}

function useReduce() {
  return (
    Iterator.from([new A()]).reduce((a, x) => x).execute() +
    Iterator.from([new B()]).reduce((a, y) => y).run()
  );
}

// Iterator.forEach / some / every bind callback element params.
function useForEach() {
  Iterator.from([new A()]).forEach((x) => x.execute());
  Iterator.from([new B()]).forEach((y) => y.run());
}

function useSomeEvery() {
  return (
    Iterator.from([new A()]).some((x) => x.execute() > 0) +
    Iterator.from([new B()]).some((y) => y.run() > 0) +
    Iterator.from([new A()]).every((x) => x.execute() > 0) +
    Iterator.from([new B()]).every((y) => y.run() > 0)
  );
}

function usePreservesB() {
  return (
    Iterator.from([0]).flatMap(() => [new B()]).next().value.run() +
    Iterator.from([new B()]).find((y) => true).run() +
    Iterator.from([new B()]).reduce((a, y) => y).run()
  );
}
