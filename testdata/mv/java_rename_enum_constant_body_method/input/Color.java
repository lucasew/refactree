package demo;

public enum Color {
  RED {
    @Override
    public int code() {
      return 1;
    }
  },
  GREEN {
    @Override
    public int code() {
      return 2;
    }
  };

  public abstract int code();

  public int stay() {
    return 0;
  }
}
