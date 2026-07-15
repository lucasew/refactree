package demo;

import java.util.function.Supplier;

public class Main {
  public static int use(Box b, Other o) {
    Supplier<Integer> sb = b::helper;
    Supplier<Integer> so = o::helper;
    return sb.get() + so.get() + b.stay();
  }
}
