package demo;

import java.lang.Appendable;
import java.io.IOException;

public class Use {
  void write(Appendable appendable) throws IOException {
    appendable.append('x');
    appendable.append("hi");
  }

  void useCustom(Custom c) throws IOException {
    c.fuzz902('y');
  }
}
