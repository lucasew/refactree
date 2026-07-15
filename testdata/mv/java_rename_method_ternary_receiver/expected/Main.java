package demo;

public class Main {
  public static int use(boolean f, Box a, Box b) {
    return (f ? a : b).assist() + a.stay();
  }
}
