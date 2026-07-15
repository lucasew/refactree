package demo;

public record Coord(int x, int y) {
  public Coord {
    if (x < 0) {
      throw new IllegalArgumentException();
    }
  }

  public int sum() {
    return x + y;
  }
}
