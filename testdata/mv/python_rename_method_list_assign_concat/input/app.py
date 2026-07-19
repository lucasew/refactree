from collections import deque


class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_plus_eq():
    xs = []
    ys = []
    xs += [A()]
    ys += [B()]
    return xs[0].run() + ys[0].run()


def use_plus_eq_for():
    xs = []
    ys = []
    xs += [A()]
    ys += [B()]
    n = 0
    for a in xs:
        n += a.run()
    for b in ys:
        n += b.run()
    return n


def use_plus_eq_var():
    xs = []
    ys = []
    xs += [A()]
    ys += [B()]
    a = xs[0]
    b = ys[0]
    return a.run() + b.run()


def use_assign_concat():
    xs = []
    ys = []
    xs = xs + [A()]
    ys = ys + [B()]
    return xs[0].run() + ys[0].run()


def use_assign_concat_for():
    xs = []
    ys = []
    xs = xs + [A()]
    ys = ys + [B()]
    n = 0
    for a in xs:
        n += a.run()
    for b in ys:
        n += b.run()
    return n


def use_assign_concat_new():
    xs = [A()]
    ys = [B()]
    zs = xs + [A()]
    ws = ys + [B()]
    return zs[0].run() + ws[0].run()


def use_star_concat():
    xs = []
    ys = []
    xs = [*xs, A()]
    ys = [*ys, B()]
    a = xs[0]
    b = ys[0]
    return a.run() + b.run()


def use_plus_eq_tuple():
    xs = list()
    ys = list()
    xs += (A(),)
    ys += (B(),)
    return xs[0].run() + ys[0].run()


def use_plus_eq_deque():
    xs = deque()
    ys = deque()
    xs += [A()]
    ys += [B()]
    return xs[0].run() + ys[0].run()


def use_preserves_b():
    ys = []
    ys += [B()]
    return ys[0].run()
