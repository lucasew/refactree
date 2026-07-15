package demo;

public class Outer {
  public static class Box {
    public int n;

    public Box(int n) {
      this.n = n;
    }
  }

  public Box make() {
    return new Box(1);
  }
}
