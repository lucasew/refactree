package demo;

public class Crate {
  public int n;

  public Crate(int n) {
    this.n = n;
  }

  public Crate() {
    this(0);
  }

  public static Crate create() {
    return new Crate();
  }
}
