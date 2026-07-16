class A {
  execute(): number {
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
  a.execute()
  b.run()
}
