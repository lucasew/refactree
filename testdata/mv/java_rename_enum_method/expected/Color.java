package demo;

public enum Color {
  RED,
  GREEN;

  public int rank() {
    return ordinal();
  }

  public int stay() {
    return 0;
  }
}
