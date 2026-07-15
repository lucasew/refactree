package demo;

public enum Hue {
  RED(1), GREEN(2);

  private final int code;

  Hue(int code) {
    this.code = code;
  }

  public int getCode() {
    return code;
  }
}
