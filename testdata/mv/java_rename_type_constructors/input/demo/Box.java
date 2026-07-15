package demo;

public class Box {
  public int n;

  public Box(int n) {
    this.n = n;
  }

  public Box() {
    this(0);
  }

  public static Box create() {
    return new Box();
  }
}
