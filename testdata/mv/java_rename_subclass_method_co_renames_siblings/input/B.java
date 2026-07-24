package demo;

public class B extends Base {
  @Override
  public void work() {}

  public void call(Base b) {
    b.work();
  }
}
