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

function useEmptyReduceInit() {
  return (
    [].reduce((a, b) => a, new A()).execute() +
    [].reduce((a, b) => a, new B()).run()
  );
}

function useEmptyReduceRightInit() {
  return (
    [].reduceRight((a, b) => a, new A()).execute() +
    [].reduceRight((a, b) => a, new B()).run()
  );
}

function useEmptyReduceInitLocal() {
  const a = [].reduce((acc, x) => acc, new A());
  const b = [].reduce((acc, x) => acc, new B());
  return a.execute() + b.run();
}

function useEmptyReduceInitBare() {
  return (
    [].reduce((a, b) => a, new A()).execute() + [].reduce((a, b) => a, new B()).run()
  );
}

function useIdent() {
  const a = new A();
  const b = new B();
  return [].reduce((acc, x) => acc, a).execute() + [].reduce((acc, x) => acc, b).run();
}

function usePreservesB() {
  return (
    [].reduce((a, b) => a, new B()).run() +
    [].reduceRight((a, b) => a, new B()).run()
  );
}
