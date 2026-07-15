package demo;

public class Main {
  public static int use() {
    return Outer.Nested.helper() + Outer.Nested.stay();
  }
}
