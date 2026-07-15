package demo;

interface Worker {
  int helper();
  int stay();
}

public enum Kind implements Worker {
  A {
    public int helper() { return 1; }
    public int stay() { return 2; }
  };
}

class Use {
  static int use(Worker w) { return w.helper() + w.stay(); }
  static int useKind(Kind k) { return k.helper() + k.stay(); }
}
