package demo;

public class Outer {
  public static class Nested {
    public static int assist() {
      return 1;
    }

    public static int stay() {
      return 2;
    }
  }

  public int use() {
    return Nested.assist() + Nested.stay();
  }
}
