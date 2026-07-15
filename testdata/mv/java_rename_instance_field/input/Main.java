package demo;

public class Main {
  public int value = 1;

  public int getValue() {
    return this.value;
  }

  public void bump(Main m) {
    m.value = m.value + 1;
  }

  public static int use(Main m) {
    return m.value;
  }

  public static void main(String[] args) {
    Main m = new Main();
    System.out.println(m.value);
    System.out.println(m.getValue());
  }
}
