package demo;

public class Task implements Worker {
  @Override
  public void work() {}
  public String label() { return name(); }
}
