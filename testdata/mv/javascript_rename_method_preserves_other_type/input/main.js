export class A {
  run() {
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
  return a.run();
}

export function useB() {
  const b = new B();
  return b.run();
}
