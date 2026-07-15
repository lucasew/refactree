package demo;

import java.util.function.Supplier;

public class Main {
  public static int use(Box b) {
    Supplier<Integer> s = b::assist;
    return s.get() + b.stay();
  }
}
