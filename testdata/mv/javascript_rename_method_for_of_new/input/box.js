class A {
  run() {
    return 1
  }
}

class B {
  run() {
    return 2
  }
}

function use() {
  for (const x of [new A()]) {
    x.run()
  }
  for (const y of [new B()]) {
    y.run()
  }
}
