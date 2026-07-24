package demo;

import java.lang.Appendable;
import java.io.IOException;

public class Custom implements Appendable {
  @Override
  public Appendable append(CharSequence csq) throws IOException {
    return this;
  }

  @Override
  public Appendable append(CharSequence csq, int start, int end) throws IOException {
    return this;
  }

  @Override
  public Appendable append(char c) throws IOException {
    return this;
  }
}
