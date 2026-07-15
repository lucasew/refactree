package demo;

public class Box {
  public static int use() {
    Worker w = new Worker() {
      public int assist() {
        return 1;
      }

      public int stay() {
        return 2;
      }
    };
    return w.assist() + w.stay();
  }
}
