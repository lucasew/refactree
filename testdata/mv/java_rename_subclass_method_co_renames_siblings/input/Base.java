package demo;

public abstract class Base {
  public abstract void work();

  private final class Nested extends Base {
    @Override
    public void work() {
      Base.this.work();
    }
  }

  public static Base anon() {
    return new Base() {
      @Override
      public void work() {}
    };
  }
}
