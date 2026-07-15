package demo;

public enum Color {
  RED(1), GREEN(2);

  private final int code;

  Color(int code) {
    this.code = code;
  }

  public int getCode() {
    return code;
  }
}
