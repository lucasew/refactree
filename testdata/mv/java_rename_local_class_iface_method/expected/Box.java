package demo;

public class Box {
  public static int use() {
    class Local implements Worker {
      public int assist() {
        return 1;
      }

      public int stay() {
        return 2;
      }
    }
    Worker w = new Local();
    return w.assist() + w.stay();
  }
}
