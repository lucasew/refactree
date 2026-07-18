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

function useSetForEach() {
  let n = 0;
  new Set([new A()]).forEach((va) => {
    n += va.execute();
  });
  new Set([new B()]).forEach((vb) => {
    n += vb.run();
  });
  return n;
}

function useSetForEachBare() {
  let n = 0;
  new Set([new A()]).forEach(va => {
    n += va.execute();
  });
  new Set([new B()]).forEach(vb => {
    n += vb.run();
  });
  return n;
}

function useSetForEachLocal() {
  const sa = new Set([new A()]);
  const sb = new Set([new B()]);
  let n = 0;
  sa.forEach((va) => {
    n += va.execute();
  });
  sb.forEach((vb) => {
    n += vb.run();
  });
  return n;
}

function useSetForEachFunc() {
  let n = 0;
  new Set([new A()]).forEach(function (va) {
    n += va.execute();
  });
  new Set([new B()]).forEach(function (vb) {
    n += vb.run();
  });
  return n;
}

function useSetForEachArrayLocal() {
  const as = [new A()];
  const bs = [new B()];
  let n = 0;
  new Set(as).forEach((va) => {
    n += va.execute();
  });
  new Set(bs).forEach((vb) => {
    n += vb.run();
  });
  return n;
}

function useIdent() {
  const a = new A();
  const b = new B();
  let n = 0;
  new Set([a]).forEach((va) => {
    n += va.execute();
  });
  new Set([b]).forEach((vb) => {
    n += vb.run();
  });
  return n;
}

function usePreservesB() {
  let n = 0;
  new Set([new B()]).forEach((vb) => {
    n += vb.run();
  });
  return n;
}
