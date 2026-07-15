export class A {
  execute() {
    return this.runHelper();
  }

  runHelper() {
    return 1;
  }
}

export class B {
  run() {
    return 2;
  }
}

export function useA() {
  const a = new A();
  return a.execute();
}

export function useB() {
  const b = new B();
  return b.run();
}
