package demo;

public class Box {
  public static int use() {
    Base b = new Base() {
      public int helper() {
        return 10;
      }
    };
    return b.helper() + b.stay();
  }
}
