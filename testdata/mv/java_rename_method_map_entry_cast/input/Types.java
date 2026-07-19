package demo;

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
  public static int useCastGetValue(Map.Entry<String, A> ea, Map.Entry<String, B> eb) {
    return ((Map.Entry<String, A>) ea).getValue().run()
        + ((Map.Entry<String, B>) eb).getValue().run();
  }

  public static int useCastVar(Map.Entry<String, A> ea, Map.Entry<String, B> eb) {
    var xa = (Map.Entry<String, A>) ea;
    var xb = (Map.Entry<String, B>) eb;
    return xa.getValue().run() + xb.getValue().run();
  }

  public static int useCastSetValue(Map.Entry<String, A> ea, Map.Entry<String, B> eb) {
    return ((Map.Entry<String, A>) ea).setValue(new A()).run()
        + ((Map.Entry<String, B>) eb).setValue(new B()).run();
  }

  public static int usePlain(Map.Entry<String, A> ea, Map.Entry<String, B> eb) {
    return ea.getValue().run() + eb.getValue().run();
  }
}
