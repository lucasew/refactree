package demo;

import java.lang.Appendable;
import java.io.IOException;

public class Custom implements Appendable {
  @Override
  public Appendable fuzz902(CharSequence csq) throws IOException {
    return this;
  }

  @Override
  public Appendable fuzz902(CharSequence csq, int start, int end) throws IOException {
    return this;
  }

  @Override
  public Appendable fuzz902(char c) throws IOException {
    return this;
  }
}
