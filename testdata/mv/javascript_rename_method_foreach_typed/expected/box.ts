export class A {
  execute(): number {
    return 1;
  }
}

export class B {
  run(): number {
    return 2;
  }
}

export function use(xs: A[], ys: Array<B>, zs: ReadonlyArray<A>): number {
  let n = 0;
  xs.forEach(a => {
    n += a.execute();
  });
  ys.forEach((b) => {
    n += b.run();
  });
  zs.map(a => a.execute()).forEach(x => {
    n += x;
  });
  return n;
}
