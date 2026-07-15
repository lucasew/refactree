package demo;

public enum Color {
  RED,
  GREEN;

  public int code() {
    return ordinal();
  }

  public int stay() {
    return 0;
  }
}
