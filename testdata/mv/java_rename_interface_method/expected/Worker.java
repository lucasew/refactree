package demo;

public interface Worker {
  void run();
  default String name() { return "worker"; }
}
