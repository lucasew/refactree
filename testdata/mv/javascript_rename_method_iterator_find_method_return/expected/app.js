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

class BoxA {
  a = new A();
  get() {
    return this.a;
  }
}

class BoxB {
  b = new B();
  get() {
    return this.b;
  }
}

function useFind() {
  return (
    Iterator.from([new BoxA().get()]).find((x) => true).execute() +
    Iterator.from([new BoxB().get()]).find((x) => true).run()
  );
}

function useFindLast() {
  return (
    Iterator.from([new BoxA().get()]).findLast((x) => true).execute() +
    Iterator.from([new BoxB().get()]).findLast((x) => true).run()
  );
}

function useFindAssign() {
  const xa = Iterator.from([new BoxA().get()]).find((x) => true);
  const xb = Iterator.from([new BoxB().get()]).find((x) => true);
  return xa.execute() + xb.run();
}

function useFlatMapToArray() {
  return (
    Iterator.from([new BoxA().get()])
      .flatMap((x) => [x])
      .toArray()[0].execute() +
    Iterator.from([new BoxB().get()])
      .flatMap((x) => [x])
      .toArray()[0].run()
  );
}

function useFilterToArray() {
  return (
    Iterator.from([new BoxA().get()])
      .filter((x) => true)
      .toArray()[0].execute() +
    Iterator.from([new BoxB().get()])
      .filter((x) => true)
      .toArray()[0].run()
  );
}

function useReduce() {
  return (
    Iterator.from([new BoxA().get()]).reduce((a, x) => a).execute() +
    Iterator.from([new BoxB().get()]).reduce((a, x) => a).run()
  );
}

function useArrayFind() {
  return (
    [new BoxA().get()].find((x) => true).execute() +
    [new BoxB().get()].find((x) => true).run()
  );
}

function useArrayFlatMap() {
  return (
    [new BoxA().get()].flatMap((x) => [x])[0].execute() +
    [new BoxB().get()].flatMap((x) => [x])[0].run()
  );
}

function useArrayReduceInit() {
  return (
    [new BoxA().get()].reduce((a, b) => a, new BoxA().get()).execute() +
    [new BoxB().get()].reduce((a, b) => a, new BoxB().get()).run()
  );
}

function useClass() {
  return (
    Iterator.from([new A()]).find((x) => true).execute() +
    Iterator.from([new B()]).find((x) => true).run() +
    Iterator.from([new A()])
      .flatMap((x) => [x])
      .toArray()[0].execute() +
    Iterator.from([new B()])
      .flatMap((x) => [x])
      .toArray()[0].run()
  );
}

function usePreservesB() {
  return (
    Iterator.from([new BoxB().get()]).find((x) => true).run() +
    Iterator.from([new BoxB().get()])
      .flatMap((x) => [x])
      .toArray()[0].run() +
    [new BoxB().get()].find((x) => true).run()
  );
}
