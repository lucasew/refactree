package demo;

public class Box {
  public static int use() {
    class Local implements Worker {
      public int helper() {
        return 1;
      }

      public int stay() {
        return 2;
      }
    }
    Worker w = new Local();
    return w.helper() + w.stay();
  }
}
