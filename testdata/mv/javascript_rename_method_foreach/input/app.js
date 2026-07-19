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

function useMapForEach() {
  let n = 0;
  new Map([["k", new A()]]).forEach((va) => {
    n += va.run();
  });
  new Map([["k", new B()]]).forEach((vb) => {
    n += vb.run();
  });
  return n;
}

function useMapForEachBare() {
  let n = 0;
  new Map([["k", new A()]]).forEach(va => {
    n += va.run();
  });
  new Map([["k", new B()]]).forEach(vb => {
    n += vb.run();
  });
  return n;
}

function useMapForEachLocal() {
  const ma = new Map([["k", new A()]]);
  const mb = new Map([["k", new B()]]);
  let n = 0;
  ma.forEach((va) => {
    n += va.run();
  });
  mb.forEach((vb) => {
    n += vb.run();
  });
  return n;
}

function useMapForEachKeyVal() {
  let n = 0;
  new Map([["k", new A()]]).forEach((va, k) => {
    n += va.run();
  });
  new Map([["k", new B()]]).forEach((vb, k) => {
    n += vb.run();
  });
  return n;
}

function useMapForEachFunc() {
  let n = 0;
  new Map([["k", new A()]]).forEach(function (va) {
    n += va.run();
  });
  new Map([["k", new B()]]).forEach(function (vb) {
    n += vb.run();
  });
  return n;
}

function useArrayForEach() {
  let n = 0;
  [new A()].forEach((va) => {
    n += va.run();
  });
  [new B()].forEach((vb) => {
    n += vb.run();
  });
  return n;
}

function useArrayForEachLocal() {
  const as = [new A()];
  const bs = [new B()];
  let n = 0;
  as.forEach((va) => {
    n += va.run();
  });
  bs.forEach((vb) => {
    n += vb.run();
  });
  return n;
}

function useArrayForEachSlice() {
  let n = 0;
  [new A()].slice().forEach((va) => {
    n += va.run();
  });
  [new B()].slice().forEach((vb) => {
    n += vb.run();
  });
  return n;
}

function useIdent() {
  const a = new A();
  const b = new B();
  let n = 0;
  new Map([["k", a]]).forEach((va) => {
    n += va.run();
  });
  new Map([["k", b]]).forEach((vb) => {
    n += vb.run();
  });
  [a].forEach((xa) => {
    n += xa.run();
  });
  [b].forEach((xb) => {
    n += xb.run();
  });
  return n;
}

function usePreservesB() {
  let n = 0;
  new Map([["k", new B()]]).forEach((vb) => {
    n += vb.run();
  });
  [new B()].forEach((vb) => {
    n += vb.run();
  });
  return n;
}
