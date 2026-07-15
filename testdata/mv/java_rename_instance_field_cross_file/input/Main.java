package demo;

public class Main {
  public static int use(Box b) {
    return b.value;
  }

  public static void main(String[] args) {
    Box b = new Box();
    System.out.println(b.value);
    b.value = 3;
  }
}
