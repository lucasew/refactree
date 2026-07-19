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

// Class regression — already solid.
function useClassObjectValuesForOf() {
  let n = 0;
  for (const ca of Object.values({ k: new A() })) {
    n += ca.run();
  }
  for (const cb of Object.values({ k: new B() })) {
    n += cb.run();
  }
  return n;
}

function useClassArrayFromForOf() {
  let n = 0;
  for (const ca of Array.from([new A()])) {
    n += ca.run();
  }
  for (const cb of Array.from([new B()])) {
    n += cb.run();
  }
  return n;
}

function useClassMapForOf() {
  let n = 0;
  for (const ca of [new A()].map((x) => x)) {
    n += ca.run();
  }
  for (const cb of [new B()].map((x) => x)) {
    n += cb.run();
  }
  return n;
}

function useClassFlatForOf() {
  let n = 0;
  for (const ca of [new A()].flat()) {
    n += ca.run();
  }
  for (const cb of [new B()].flat()) {
    n += cb.run();
  }
  return n;
}

function useClassFlatMapForOf() {
  let n = 0;
  for (const ca of [new A()].flatMap((x) => [x])) {
    n += ca.run();
  }
  for (const cb of [new B()].flatMap((x) => [x])) {
    n += cb.run();
  }
  return n;
}

// Method-return for-of under foreign same-leaf (already solid via array-source peels).
function useMRObjectValuesForOf() {
  let n = 0;
  for (const ma of Object.values({ k: new BoxA().get() })) {
    n += ma.run();
  }
  for (const mb of Object.values({ k: new BoxB().get() })) {
    n += mb.run();
  }
  return n;
}

function useMRArrayFromForOf() {
  let n = 0;
  for (const ma of Array.from([new BoxA().get()])) {
    n += ma.run();
  }
  for (const mb of Array.from([new BoxB().get()])) {
    n += mb.run();
  }
  return n;
}

function useMRMapForOf() {
  let n = 0;
  for (const ma of [new BoxA().get()].map((x) => x)) {
    n += ma.run();
  }
  for (const mb of [new BoxB().get()].map((x) => x)) {
    n += mb.run();
  }
  return n;
}

function useMRFlatForOf() {
  let n = 0;
  for (const ma of [new BoxA().get()].flat()) {
    n += ma.run();
  }
  for (const mb of [new BoxB().get()].flat()) {
    n += mb.run();
  }
  return n;
}

function useMRFlatMapForOf() {
  let n = 0;
  for (const ma of [new BoxA().get()].flatMap((x) => [x])) {
    n += ma.run();
  }
  for (const mb of [new BoxB().get()].flatMap((x) => [x])) {
    n += mb.run();
  }
  return n;
}

function useMRLocalBoxForOf() {
  let n = 0;
  const ba = new BoxA();
  const bb = new BoxB();
  for (const ma of Object.values({ k: ba.get() })) {
    n += ma.run();
  }
  for (const mb of Object.values({ k: bb.get() })) {
    n += mb.run();
  }
  for (const ma of Array.from([ba.get()])) {
    n += ma.run();
  }
  for (const mb of Array.from([bb.get()])) {
    n += mb.run();
  }
  return n;
}

function usePreservesB() {
  let n = 0;
  for (const mb of Object.values({ k: new BoxB().get() })) {
    n += mb.run();
  }
  for (const mb of Array.from([new BoxB().get()])) {
    n += mb.run();
  }
  for (const mb of [new BoxB().get()].map((x) => x)) {
    n += mb.run();
  }
  for (const mb of [new BoxB().get()].flat()) {
    n += mb.run();
  }
  for (const mb of [new BoxB().get()].flatMap((x) => [x])) {
    n += mb.run();
  }
  return n;
}
