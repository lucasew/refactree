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

function useSpreadProp() {
  return { ...{ k: new A() } }.k.run() + { ...{ k: new B() } }.k.run();
}

function useSpreadLocal() {
  const oa = { k: new A() };
  const ob = { k: new B() };
  return { ...oa }.k.run() + { ...ob }.k.run();
}

function useSpreadAssign() {
  const a = { ...{ k: new A() } }.k;
  const b = { ...{ k: new B() } }.k;
  return a.run() + b.run();
}

function useMultiSpread() {
  return (
    { ...{ k: new A() }, ...{ m: new A() } }.k.run() +
    { ...{ k: new B() }, ...{ m: new B() } }.k.run()
  );
}
