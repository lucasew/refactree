package demo;

public class Main {
  public int amount = 1;

  public int getValue() {
    return this.amount;
  }

  public void bump(Main m) {
    m.amount = m.amount + 1;
  }

  public static int use(Main m) {
    return m.amount;
  }

  public static void main(String[] args) {
    Main m = new Main();
    System.out.println(m.amount);
    System.out.println(m.getValue());
  }
}
