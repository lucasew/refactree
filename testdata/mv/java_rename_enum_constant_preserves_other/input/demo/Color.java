package demo;

public enum Color {
  RED, GREEN;

  public static int mix() {
    return RED.ordinal() + GREEN.ordinal();
  }
}
