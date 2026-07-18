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

function useAtLiteral() {
  return [new A()].at(0).execute() + [new B()].at(0).run();
}

function useAtLocal() {
  const as = [new A()];
  const bs = [new B()];
  return as.at(0).execute() + bs.at(0).run();
}

function useAtAssign() {
  const xa = [new A()].at(0);
  const xb = [new B()].at(0);
  return xa.execute() + xb.run();
}

function useAtNeg() {
  return [new A()].at(-1).execute() + [new B()].at(-1).run();
}

function useAtArrayFrom() {
  return (
    Array.from([new A()]).at(0).execute() + Array.from([new B()]).at(0).run()
  );
}

function useAtArrayOf() {
  return Array.of(new A()).at(0).execute() + Array.of(new B()).at(0).run();
}

function useAtObjectValues() {
  return (
    Object.values({ k: new A() }).at(0).execute() +
    Object.values({ k: new B() }).at(0).run()
  );
}

function useIdent() {
  const a = new A();
  const b = new B();
  return [a].at(0).execute() + [b].at(0).run();
}

function usePreservesB() {
  return (
    [new B()].at(0).run() +
    Array.from([new B()]).at(0).run() +
    Object.values({ k: new B() }).at(0).run()
  );
}
