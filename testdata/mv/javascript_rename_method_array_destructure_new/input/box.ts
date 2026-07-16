class A {
  run(): number {
    return 1
  }
}

class B {
  run(): number {
    return 2
  }
}

function use() {
  const [a, b] = [new A(), new B()]
  a.run()
  b.run()
}
