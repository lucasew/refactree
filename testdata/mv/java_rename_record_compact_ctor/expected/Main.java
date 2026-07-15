package demo;

public class Main {
  public static int use(Coord p) {
    return p.sum() + new Coord(1, 2).sum();
  }
}
