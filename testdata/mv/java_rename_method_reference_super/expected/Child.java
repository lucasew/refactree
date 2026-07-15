package demo;

import java.util.function.Supplier;

public class Child extends Base {
  public int use() {
    Supplier<Integer> s = super::assist;
    return s.get() + super.stay();
  }
}
