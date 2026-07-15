package demo;

import java.util.function.Supplier;

public class Box {
  public static Box make() { return new Box(); }
  public int value() { return 1; }
  public static Supplier<Box> factory() {
    return Box::make;
  }
  public Supplier<Integer> valRef() {
    return this::value;
  }
}
