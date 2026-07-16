package demo;

import java.util.function.Supplier;

public class Main {
  public static int use() {
    Supplier<Integer> s = Box.make()::assist;
    return s.get() + Box.make().stay();
  }
}
