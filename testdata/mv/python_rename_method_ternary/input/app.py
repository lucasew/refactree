class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_elem(a: A, x: A, b: B, y: B, c: bool):
    return (a if c else x).run() + (b if c else y).run()


def use_list(aa: list[A], ca: list[A], bb: list[B], cb: list[B], c: bool):
    return (aa if c else ca)[0].run() + (bb if c else cb)[0].run()


def use_list_same(aa: list[A], bb: list[B], c: bool):
    return (aa if c else aa)[0].run() + (bb if c else bb)[0].run()


def use_assign(a: A, x: A, b: B, y: B, c: bool):
    xa = a if c else x
    xb = b if c else y
    return xa.run() + xb.run()


def use_ctor(c: bool):
    return (A() if c else A()).run() + (B() if c else B()).run()
