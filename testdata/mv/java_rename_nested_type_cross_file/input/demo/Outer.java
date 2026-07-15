package demo;

public class Outer {
  public static class Inner {
    public int n;

    public Inner(int n) {
      this.n = n;
    }

    public int value() {
      return n;
    }
  }

  public static Inner make(int n) {
    return new Inner(n);
  }
}
