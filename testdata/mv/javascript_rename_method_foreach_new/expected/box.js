export class A {
  execute() {
    return 1;
  }
}

export class B {
  run() {
    return 2;
  }
}

export function use() {
  const xs = [new A()];
  const ys = [new B()];
  let n = 0;
  xs.forEach(a => {
    n += a.execute();
  });
  ys.filter(b => b.run() > 0).forEach(b => {
    n += b.run();
  });
  return n;
}
