class A {
  renamed() { return 1; }
}
class B {
  helper() { return 2; }
}

class BoxA {
  a = new A();
  static sa = new A();
}
class BoxB {
  b = new B();
  static sb = new B();
}

export function useNew() {
  return new BoxA().a.renamed() + new BoxB().b.helper();
}
export function useStatic() {
  return BoxA.sa.renamed() + BoxB.sb.helper();
}
export function usePreservesB() {
  return new BoxB().b.helper() + BoxB.sb.helper();
}
