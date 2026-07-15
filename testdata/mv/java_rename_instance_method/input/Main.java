package demo;

public class Main {
  public int getValue() {
    return 1;
  }

  public int twice() {
    return this.getValue();
  }

  public static void use(Main m) {
    System.out.println(m.getValue());
  }

  public static void main(String[] args) {
    Main m = new Main();
    System.out.println(m.getValue());
    System.out.println(m.twice());
  }
}
