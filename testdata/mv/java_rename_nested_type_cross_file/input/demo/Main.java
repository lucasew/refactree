package demo;

public class Main {
  public static int run() {
    Outer.Inner i = Outer.make(1);
    return i.value();
  }
}
