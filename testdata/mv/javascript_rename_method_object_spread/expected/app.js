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

function useSpreadProp() {
  return { ...{ k: new A() } }.k.execute() + { ...{ k: new B() } }.k.run();
}

function useSpreadLocal() {
  const oa = { k: new A() };
  const ob = { k: new B() };
  return { ...oa }.k.execute() + { ...ob }.k.run();
}

function useSpreadAssign() {
  const a = { ...{ k: new A() } }.k;
  const b = { ...{ k: new B() } }.k;
  return a.execute() + b.run();
}

function useMultiSpread() {
  return (
    { ...{ k: new A() }, ...{ m: new A() } }.k.execute() +
    { ...{ k: new B() }, ...{ m: new B() } }.k.run()
  );
}
