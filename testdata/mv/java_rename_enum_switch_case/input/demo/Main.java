package demo;

public class Main {
  public static int use(Color c) {
    return switch (c) {
      case RED -> 1;
      case GREEN -> 2;
      case BLUE -> 3;
    };
  }

  public static Color pick() {
    return Color.RED;
  }
}
