package demo;

public class B extends Base {
  @Override
  public void run() {}

  public void call(Base b) {
    b.run();
  }
}
