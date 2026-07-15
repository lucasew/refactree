package demo;

public class Box {
  public static int use() {
    Worker w = new Worker() {
      public int helper() {
        return 1;
      }

      public int stay() {
        return 2;
      }
    };
    return w.helper() + w.stay();
  }
}
