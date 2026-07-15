package demo;

import java.util.function.Supplier;

public class Main {
  public static Supplier<Box> f() {
    return Box::create;
  }
}
