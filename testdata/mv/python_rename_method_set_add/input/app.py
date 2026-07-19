class A:
    def run(self) -> int:
        return 1


class B:
    def run(self) -> int:
        return 2


def use_set_add_next():
    xs = set()
    ys = set()
    xs.add(A())
    ys.add(B())
    return next(iter(xs)).run() + next(iter(ys)).run()


def use_set_add_next_var():
    xs = set()
    ys = set()
    xs.add(A())
    ys.add(B())
    a = next(iter(xs))
    b = next(iter(ys))
    return a.run() + b.run()


def use_set_add_for():
    xs = set()
    ys = set()
    xs.add(A())
    ys.add(B())
    n = 0
    for a in xs:
        n += a.run()
    for b in ys:
        n += b.run()
    return n


def use_set_literal_add():
    xs = {A()}
    ys = {B()}
    return next(iter(xs)).run() + next(iter(ys)).run()


def use_preserves_b():
    ys = set()
    ys.add(B())
    return next(iter(ys)).run()
