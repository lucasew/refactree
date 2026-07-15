import java.util.function.Supplier;
import java.util.function.ToIntFunction;

public class Main {
  public static int use(Box b) {
    ToIntFunction<Box> f = Box::helper;
    Supplier<Integer> s = b::helper;
    return f.applyAsInt(b) + s.get() + b.helper() + b.stay();
  }
}
