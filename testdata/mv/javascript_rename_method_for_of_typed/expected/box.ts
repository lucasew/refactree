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

function use(xs: A[], ys: Array<B>) {
  for (const a of xs) {
    a.execute()
  }
  for (const b of ys) {
    b.run()
  }
}
