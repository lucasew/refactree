package demo;

public abstract class Base {
  public abstract void run();

  private final class Nested extends Base {
    @Override
    public void run() {
      Base.this.run();
    }
  }

  public static Base anon() {
    return new Base() {
      @Override
      public void run() {}
    };
  }
}
