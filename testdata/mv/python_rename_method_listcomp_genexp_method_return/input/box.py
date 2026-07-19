class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


class BoxA:
    a: A

    def __init__(self, a: A):
        self.a = a

    def get(self) -> A:
        return self.a


class BoxB:
    b: B

    def __init__(self, b: B):
        self.b = b

    def get(self) -> B:
        return self.b


def use_listcomp_mr(ba: BoxA, bb: BoxB) -> int:
    xs = [ba.get() for _ in range(1)]
    ys = [bb.get() for _ in range(1)]
    return xs[0].run() + ys[0].run()


def use_listcomp_class() -> int:
    xs = [A() for _ in range(1)]
    ys = [B() for _ in range(1)]
    return xs[0].run() + ys[0].run()


def use_listcomp_inline_mr(ba: BoxA, bb: BoxB) -> int:
    return [ba.get() for _ in range(1)][0].run() + [bb.get() for _ in range(1)][0].run()


def use_listcomp_inline_class() -> int:
    return [A() for _ in range(1)][0].run() + [B() for _ in range(1)][0].run()


def use_genexp_next_mr(ba: BoxA, bb: BoxB) -> int:
    xs = (ba.get() for _ in range(1))
    ys = (bb.get() for _ in range(1))
    return next(xs).run() + next(ys).run()


def use_genexp_next_class() -> int:
    xs = (A() for _ in range(1))
    ys = (B() for _ in range(1))
    return next(xs).run() + next(ys).run()


def use_genexp_inline_mr(ba: BoxA, bb: BoxB) -> int:
    return next(ba.get() for _ in range(1)).run() + next(bb.get() for _ in range(1)).run()


def use_genexp_inline_class() -> int:
    return next(A() for _ in range(1)).run() + next(B() for _ in range(1)).run()


def use_setcomp_mr(ba: BoxA, bb: BoxB) -> int:
    xs = {ba.get() for _ in range(1)}
    ys = {bb.get() for _ in range(1)}
    return next(iter(xs)).run() + next(iter(ys)).run()


def use_setcomp_class() -> int:
    xs = {A() for _ in range(1)}
    ys = {B() for _ in range(1)}
    return next(iter(xs)).run() + next(iter(ys)).run()
