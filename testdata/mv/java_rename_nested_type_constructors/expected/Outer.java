package demo;

public class Outer {
  public static class Crate {
    public int n;

    public Crate(int n) {
      this.n = n;
    }
  }

  public Crate make() {
    return new Crate(1);
  }
}
