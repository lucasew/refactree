import java.util.function.Supplier;

public class Main {
  public static int use(Box b, Stay s) {
    Supplier<Integer> sb = b::assist;
    Supplier<Integer> ss = s::helper;
    return sb.get() + ss.get() + b.assist() + s.helper();
  }
}
