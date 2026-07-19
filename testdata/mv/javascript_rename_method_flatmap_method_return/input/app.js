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

function useIteratorFlatMap() {
  return (
    [new BoxA().get()].values().flatMap((x) => [x]).next().value.run() +
    [new BoxB().get()].values().flatMap((x) => [x]).next().value.run()
  );
}

function useIteratorFromFlatMap() {
  return (
    Iterator.from([new BoxA().get()])
      .flatMap((x) => [x])
      .next().value.run() +
    Iterator.from([new BoxB().get()])
      .flatMap((x) => [x])
      .next().value.run()
  );
}

function useIteratorFlatMapAssign() {
  const xa = [new BoxA().get()].values().flatMap((x) => [x]).next().value;
  const xb = [new BoxB().get()].values().flatMap((x) => [x]).next().value;
  return xa.run() + xb.run();
}

function useIteratorFlatMapToArray() {
  return (
    [new BoxA().get()].values().flatMap((x) => [x]).toArray()[0].run() +
    [new BoxB().get()].values().flatMap((x) => [x]).toArray()[0].run()
  );
}

function useIteratorTakeFlatMap() {
  return (
    [new BoxA().get()]
      .values()
      .take(1)
      .flatMap((x) => [x])
      .next().value.run() +
    [new BoxB().get()]
      .values()
      .take(1)
      .flatMap((x) => [x])
      .next().value.run()
  );
}

function useSetValuesFlatMap() {
  return (
    new Set([new BoxA().get()]).values().flatMap((x) => [x]).next().value
      .run() +
    new Set([new BoxB().get()]).values().flatMap((x) => [x]).next().value.run()
  );
}

function useArrayFlatMapCtor() {
  return (
    [0].flatMap(() => [new BoxA().get()])[0].run() +
    [0].flatMap(() => [new BoxB().get()])[0].run()
  );
}

function useArrayFlatMapCtorAssign() {
  const xs = [0].flatMap(() => [new BoxA().get()]);
  const ys = [0].flatMap(() => [new BoxB().get()]);
  return xs[0].run() + ys[0].run();
}

function useArrayFlatMapCtorAt() {
  return (
    [0].flatMap((x) => [new BoxA().get()]).at(0).run() +
    [0].flatMap((x) => [new BoxB().get()]).at(0).run()
  );
}

function useClassIteratorFlatMap() {
  return (
    [new A()].values().flatMap((x) => [x]).next().value.run() +
    [new B()].values().flatMap((x) => [x]).next().value.run()
  );
}

function useClassArrayFlatMapCtor() {
  return (
    [0].flatMap(() => [new A()])[0].run() +
    [0].flatMap(() => [new B()])[0].run()
  );
}

function usePreservesB() {
  return (
    [new BoxB().get()].values().flatMap((x) => [x]).next().value.run() +
    [0].flatMap(() => [new BoxB().get()])[0].run()
  );
}
