package demo;

interface Worker {
  int assist();
  int stay();
}

public enum Kind implements Worker {
  A {
    public int assist() { return 1; }
    public int stay() { return 2; }
  };
}

class Use {
  static int use(Worker w) { return w.assist() + w.stay(); }
  static int useKind(Kind k) { return k.assist() + k.stay(); }
}
