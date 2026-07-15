package demo;

public enum Color {
  CRIMSON, GREEN;

  public static int mix() {
    return CRIMSON.ordinal() + GREEN.ordinal();
  }
}
