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

function useSome() {
  let n = 0;
  [new A()].some((va) => {
    n += va.execute();
    return false;
  });
  [new B()].some((vb) => {
    n += vb.run();
    return false;
  });
  return n;
}

function useEvery() {
  let n = 0;
  [new A()].every((va) => {
    n += va.execute();
    return true;
  });
  [new B()].every((vb) => {
    n += vb.run();
    return true;
  });
  return n;
}

function useFilterCb() {
  let n = 0;
  [new A()].filter((va) => {
    n += va.execute();
    return true;
  });
  [new B()].filter((vb) => {
    n += vb.run();
    return true;
  });
  return n;
}

function useMapCb() {
  return [new A()].map((va) => va.execute())[0] + [new B()].map((vb) => vb.run())[0];
}

function useFindCb() {
  let n = 0;
  [new A()].find((va) => {
    n += va.execute();
    return true;
  });
  [new B()].find((vb) => {
    n += vb.run();
    return true;
  });
  return n;
}

function useFindLastCb() {
  let n = 0;
  [new A()].findLast((va) => {
    n += va.execute();
    return true;
  });
  [new B()].findLast((vb) => {
    n += vb.run();
    return true;
  });
  return n;
}

function useFlatMapCb() {
  let n = 0;
  [new A()].flatMap((va) => {
    n += va.execute();
    return [va];
  });
  [new B()].flatMap((vb) => {
    n += vb.run();
    return [vb];
  });
  return n;
}

function useLocal() {
  const as = [new A()];
  const bs = [new B()];
  let n = 0;
  as.some((va) => {
    n += va.execute();
    return false;
  });
  bs.some((vb) => {
    n += vb.run();
    return false;
  });
  return n;
}

function usePreservesB() {
  let n = 0;
  [new B()].some((vb) => {
    n += vb.run();
    return false;
  });
  [new B()].map((vb) => vb.run());
  return n;
}
