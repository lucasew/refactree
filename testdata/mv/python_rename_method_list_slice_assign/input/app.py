class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_full_slice():
    xs = [None]
    ys = [None]
    xs[:] = [A()]
    ys[:] = [B()]
    return xs[0].run() + ys[0].run()


def use_empty():
    xs = []
    ys = []
    xs[:] = [A()]
    ys[:] = [B()]
    return xs[0].run() + ys[0].run()


def use_range_slice():
    xs = [None, None]
    ys = [None, None]
    xs[0:1] = [A()]
    ys[0:1] = [B()]
    return xs[0].run() + ys[0].run()


def use_tuple_rhs():
    xs = [None]
    ys = [None]
    xs[:] = (A(),)
    ys[:] = (B(),)
    return xs[0].run() + ys[0].run()


def use_for():
    xs = []
    ys = []
    xs[:] = [A()]
    ys[:] = [B()]
    s = 0
    for a in xs:
        s += a.run()
    for b in ys:
        s += b.run()
    return s


def use_preserves_b():
    ys = []
    ys[:] = [B()]
    return ys[0].run()
