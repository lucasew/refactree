package demo;

public class Outer {
  public static class Core {
    public int n;

    public Inner(int n) {
      this.n = n;
    }

    public int value() {
      return n;
    }
  }

  public static Core make(int n) {
    return new Core(n);
  }
}
