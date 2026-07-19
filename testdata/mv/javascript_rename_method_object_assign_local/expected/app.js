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

function useAssignTarget() {
  const oa = { k: new A() };
  const ob = { k: new B() };
  return Object.assign(oa).k.execute() + Object.assign(ob).k.run();
}

function useAssignTargetBracket() {
  const oa = { k: new A() };
  const ob = { k: new B() };
  return Object.assign(oa)["k"].execute() + Object.assign(ob)["k"].run();
}

function useAssignTargetValues() {
  const oa = { k: new A() };
  const ob = { k: new B() };
  return (
    Object.values(Object.assign(oa))[0].execute() +
    Object.values(Object.assign(ob))[0].run()
  );
}
