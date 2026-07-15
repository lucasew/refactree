package demo;

public class Box {
  public static int use() {
    Base b = new Base() {
      public int assist() {
        return 10;
      }
    };
    return b.assist() + b.stay();
  }
}
