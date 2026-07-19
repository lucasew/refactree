class A {
  run() {
    return 1;
  }
}

class B {
  run() {
    return 2;
  }
}

function useCreate() {
  const a = new A();
  const b = new B();
  const pa = Object.create(a);
  const pb = Object.create(b);
  return pa.run() + pb.run();
}

function useCreateInline() {
  const a = new A();
  const b = new B();
  return Object.create(a).run() + Object.create(b).run();
}

function useCreateCtor() {
  return Object.create(new A()).run() + Object.create(new B()).run();
}

function useCreateProto() {
  const pa = Object.create(A.prototype);
  const pb = Object.create(B.prototype);
  return pa.run() + pb.run();
}
