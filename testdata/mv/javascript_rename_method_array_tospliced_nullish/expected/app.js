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

function useNullToSpliced() {
  return (
    [null].toSpliced(0, 1, new A())[0].execute() +
    [null].toSpliced(0, 1, new B())[0].run()
  );
}

function useUndefinedToSpliced() {
  return (
    [undefined].toSpliced(0, 1, new A())[0].execute() +
    [undefined].toSpliced(0, 1, new B())[0].run()
  );
}

function useAssign() {
  const aa = [null].toSpliced(0, 1, new A());
  const bb = [null].toSpliced(0, 1, new B());
  return aa[0].execute() + bb[0].run();
}

function useAt() {
  return (
    [null].toSpliced(0, 1, new A()).at(0).execute() +
    [null].toSpliced(0, 1, new B()).at(0).run()
  );
}

function useForOf() {
  let n = 0;
  for (const xa of [null].toSpliced(0, 1, new A())) {
    n += xa.execute();
  }
  for (const xb of [null].toSpliced(0, 1, new B())) {
    n += xb.run();
  }
  return n;
}

function useEmptyStill() {
  return (
    [].toSpliced(0, 0, new A())[0].execute() + [].toSpliced(0, 0, new B())[0].run()
  );
}

function usePreservesB() {
  return [null].toSpliced(0, 1, new B())[0].run();
}
