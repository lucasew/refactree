public class Main {
  public enum Color { ASSIST, STAY }

  public static int use(Color c) {
    return switch (c) {
      case ASSIST -> 1;
      case STAY -> 2;
    };
  }

  public static Color pick() {
    return Color.ASSIST;
  }
}
