import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.HashSet;
import java.util.LinkedList;
import java.util.List;
import java.util.TreeSet;
import java.util.Vector;

class A {
  int run() {
    return 1;
  }
}

class B {
  int run() {
    return 2;
  }
}

class BoxA {
  A a;

  BoxA(A a) {
    this.a = a;
  }

  A get() {
    return a;
  }
}

class BoxB {
  B b;

  BoxB(B b) {
    this.b = b;
  }

  B get() {
    return b;
  }
}

class Use {
  // Diamond copy ctors: E from List.of(method-return) under foreign same-leaf.
  int useArrayList(BoxA ba, BoxB bb) {
    return new ArrayList<>(List.of(ba.get())).get(0).run()
        + new ArrayList<>(List.of(bb.get())).get(0).run();
  }

  int useLinkedList(BoxA ba, BoxB bb) {
    return new LinkedList<>(List.of(ba.get())).getFirst().run()
        + new LinkedList<>(List.of(bb.get())).getFirst().run();
  }

  int useHashSet(BoxA ba, BoxB bb) {
    return new HashSet<>(List.of(ba.get())).iterator().next().run()
        + new HashSet<>(List.of(bb.get())).iterator().next().run();
  }

  int useArrayDeque(BoxA ba, BoxB bb) {
    return new ArrayDeque<>(List.of(ba.get())).getFirst().run()
        + new ArrayDeque<>(List.of(bb.get())).getFirst().run();
  }

  int useVector(BoxA ba, BoxB bb) {
    return new Vector<>(List.of(ba.get())).firstElement().run()
        + new Vector<>(List.of(bb.get())).firstElement().run();
  }

  int useTreeSet(BoxA ba, BoxB bb) {
    return new TreeSet<>(List.of(ba.get())).first().run()
        + new TreeSet<>(List.of(bb.get())).first().run();
  }

  int useArraysAsList(BoxA ba, BoxB bb) {
    return new ArrayList<>(Arrays.asList(ba.get())).get(0).run()
        + new ArrayList<>(Arrays.asList(bb.get())).get(0).run();
  }

  int useVar(BoxA ba, BoxB bb) {
    var al = new ArrayList<>(List.of(ba.get()));
    var bl = new ArrayList<>(List.of(bb.get()));
    return al.get(0).run() + bl.get(0).run();
  }

  // Class regression — diamond List.of(new T()) already worked.
  int useClassArrayList() {
    return new ArrayList<>(List.of(new A())).get(0).run()
        + new ArrayList<>(List.of(new B())).get(0).run();
  }

  int useClassHashSet() {
    return new HashSet<>(List.of(new A())).iterator().next().run()
        + new HashSet<>(List.of(new B())).iterator().next().run();
  }

  int usePreservesB(BoxB bb) {
    return new ArrayList<>(List.of(bb.get())).get(0).run()
        + new HashSet<>(List.of(bb.get())).iterator().next().run()
        + new ArrayList<>(List.of(new B())).get(0).run();
  }
}
