package demo;

import java.util.function.Supplier;

public class Box {
  public static Box create() { return new Box(); }
  public int amount() { return 1; }
  public static Supplier<Box> factory() {
    return Box::create;
  }
  public Supplier<Integer> valRef() {
    return this::amount;
  }
}
