public class Main {
  public enum Color { HELPER, STAY }

  public static int use(Color c) {
    return switch (c) {
      case HELPER -> 1;
      case STAY -> 2;
    };
  }

  public static Color pick() {
    return Color.HELPER;
  }
}
