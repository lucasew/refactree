package demo;

public class Main {
  public static int use(Point p) {
    return p.total() + new Point(1, 2).total() + p.stay() + new Point(0, 0).stay();
  }
}
