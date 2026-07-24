package demo;

public class Obj extends Base {
  public Obj getAsRenamed(String name) {
    return null;
  }

  void use() {
    getAsRenamed("x");
  }
}
