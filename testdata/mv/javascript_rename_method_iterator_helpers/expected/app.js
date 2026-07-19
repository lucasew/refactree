class A {
  execute() {
    return 1;
  }
}

class B {
  run() {
    return 2;
  }
}

function useTakeSpread() {
  const aa = [new A()];
  const bb = [new B()];
  return (
    [...aa.values().take(1)][0].execute() + [...bb.values().take(1)][0].run()
  );
}

function useDropSpread() {
  const aa = [new A(), new A()];
  const bb = [new B(), new B()];
  return (
    [...aa.values().drop(1)][0].execute() + [...bb.values().drop(1)][0].run()
  );
}

function useToArray() {
  const aa = [new A()];
  const bb = [new B()];
  return aa.values().toArray()[0].execute() + bb.values().toArray()[0].run();
}

function useTakeToArray() {
  const aa = [new A()];
  const bb = [new B()];
  return (
    aa.values().take(1).toArray()[0].execute() +
    bb.values().take(1).toArray()[0].run()
  );
}

async function useFromAsync() {
  const aa = [new A()];
  const bb = [new B()];
  return (
    (await Array.fromAsync(aa))[0].execute() +
    (await Array.fromAsync(bb))[0].run()
  );
}
