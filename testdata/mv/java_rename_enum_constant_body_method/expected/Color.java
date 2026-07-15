package demo;

public enum Color {
  RED {
    @Override
    public int rank() {
      return 1;
    }
  },
  GREEN {
    @Override
    public int rank() {
      return 2;
    }
  };

  public abstract int rank();

  public int stay() {
    return 0;
  }
}
