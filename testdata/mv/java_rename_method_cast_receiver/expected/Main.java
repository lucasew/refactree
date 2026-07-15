package demo;

public class Main {
  public static int use(Object o) {
    return ((Box) o).assist() + ((Box) o).stay();
  }
}
