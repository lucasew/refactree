package demo;

class Base {
  public int helper() { return 1; }
  public int stay() { return 2; }
}

public class Box extends Base {
  @Override
  public int helper() { return super.helper() + 10; }

  public int use() { return helper() + stay(); }

  public static int useBox(Box b) { return b.helper() + b.stay(); }
}
