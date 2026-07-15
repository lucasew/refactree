package demo;

public interface Worker {
  void work();
  default String name() { return "worker"; }
}
