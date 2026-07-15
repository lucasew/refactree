package demo;

class Base {
  public int assist() { return 1; }
  public int stay() { return 2; }
}

public class Box extends Base {
  @Override
  public int assist() { return super.assist() + 10; }

  public int use() { return assist() + stay(); }

  public static int useBox(Box b) { return b.assist() + b.stay(); }
}
