package demo;

public class A {
  public int value = 1;

  public int get() {
    return this.value;
  }
}

class B {
  public int value = 2;

  public int get() {
    return this.value;
  }
}

class Uses {
  public static int useA(A a) {
    return a.value;
  }

  public static int useB(B b) {
    return b.value;
  }
}
