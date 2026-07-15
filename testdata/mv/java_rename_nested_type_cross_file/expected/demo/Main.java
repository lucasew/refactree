package demo;

public class Main {
  public static int run() {
    Outer.Core i = Outer.make(1);
    return i.value();
  }
}
