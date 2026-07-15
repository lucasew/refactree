package demo;

public class Main {
  public int fetchValue() {
    return 1;
  }

  public int twice() {
    return this.fetchValue();
  }

  public static void use(Main m) {
    System.out.println(m.fetchValue());
  }

  public static void main(String[] args) {
    Main m = new Main();
    System.out.println(m.fetchValue());
    System.out.println(m.twice());
  }
}
