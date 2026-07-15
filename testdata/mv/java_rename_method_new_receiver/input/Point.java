package demo;

public record Point(int x, int y) {
  public int sum() {
    return x + y;
  }

  public int stay() {
    return 0;
  }
}
