import java.util.function.Supplier;
import java.util.function.ToIntFunction;

public class Main {
  public static int use(Box b) {
    ToIntFunction<Box> f = Box::assist;
    Supplier<Integer> s = b::assist;
    return f.applyAsInt(b) + s.get() + b.assist() + b.stay();
  }
}
