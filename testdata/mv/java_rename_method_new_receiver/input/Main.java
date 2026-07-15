package demo;

public class Main {
  public static int use(Point p) {
    return p.sum() + new Point(1, 2).sum() + p.stay() + new Point(0, 0).stay();
  }
}
