package demo;

import java.util.Deque;
import java.util.List;
import java.util.Map;

public class A {
  public int run() {
    return 1;
  }
}

class B {
  public int run() {
    return 2;
  }
}

class Uses {
  public static int useGet(List<List<A>> nestedA, List<List<B>> nestedB) {
    return nestedA.get(0).get(0).run() + nestedB.get(0).get(0).run();
  }

  public static int useStream(List<List<A>> nestedA, List<List<B>> nestedB) {
    return nestedA.stream().mapToInt(rowA -> rowA.get(0).run()).sum()
        + nestedB.stream().mapToInt(rowB -> rowB.get(0).run()).sum();
  }

  public static int useMapGet(Map<String, List<A>> ma, Map<String, List<B>> mb) {
    return ma.get("k").get(0).run() + mb.get("k").get(0).run();
  }

  public static int useMapValues(Map<String, List<A>> ma, Map<String, List<B>> mb) {
    int n = 0;
    for (List<A> ga : ma.values()) {
      n += ga.get(0).run();
    }
    for (List<B> gb : mb.values()) {
      n += gb.get(0).run();
    }
    return n;
  }

  public static int useDeque(Deque<List<A>> da, Deque<List<B>> db) {
    return da.getFirst().get(0).run() + db.getFirst().get(0).run();
  }

  public static int usePreservesB(List<List<B>> nestedB) {
    return nestedB.get(0).get(0).run();
  }
}
