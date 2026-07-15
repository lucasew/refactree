package demo;

public class Main {
  public static int use(Box b) {
    return b.amount;
  }

  public static void main(String[] args) {
    Box b = new Box();
    System.out.println(b.amount);
    b.amount = 3;
  }
}
