class A {
  run() { return 1; }
}
class B {
  run() { return 2; }
}
class BoxA {
  a = new A();
  get() { return this.a; }
}
class BoxB {
  b = new B();
  get() { return this.b; }
}
async function useClassCatch() {
  return (
    (await Promise.reject(null).catch(() => new A())).run() +
    (await Promise.reject(null).catch(() => new B())).run()
  );
}
async function useMRCatch() {
  return (
    (await Promise.reject(null).catch(() => new BoxA().get())).run() +
    (await Promise.reject(null).catch(() => new BoxB().get())).run()
  );
}
async function useClassCatchAssign() {
  const csa = await Promise.reject(null).catch(() => new A());
  const csb = await Promise.reject(null).catch(() => new B());
  return csa.run() + csb.run();
}
async function useMRCatchAssign() {
  const msa = await Promise.reject(null).catch(() => new BoxA().get());
  const msb = await Promise.reject(null).catch(() => new BoxB().get());
  return msa.run() + msb.run();
}
async function usePreservesB() {
  return (await Promise.reject(null).catch(() => new BoxB().get())).run();
}
