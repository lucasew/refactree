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

function useSetForOf() {
  let n = 0;
  for (const xa of new Set([new A()])) {
    n += xa.run();
  }
  for (const xb of new Set([new B()])) {
    n += xb.run();
  }
  return n;
}

function useSetForOfLocal() {
  const sa = new Set([new A()]);
  const sb = new Set([new B()]);
  let n = 0;
  for (const xa of sa) {
    n += xa.run();
  }
  for (const xb of sb) {
    n += xb.run();
  }
  return n;
}

function useSetForOfArrayLocal() {
  const as = [new A()];
  const bs = [new B()];
  let n = 0;
  for (const xa of new Set(as)) {
    n += xa.run();
  }
  for (const xb of new Set(bs)) {
    n += xb.run();
  }
  return n;
}

function useIdent() {
  const a = new A();
  const b = new B();
  let n = 0;
  for (const xa of new Set([a])) {
    n += xa.run();
  }
  for (const xb of new Set([b])) {
    n += xb.run();
  }
  return n;
}

function usePreservesB() {
  let n = 0;
  for (const xb of new Set([new B()])) {
    n += xb.run();
  }
  return n;
}
