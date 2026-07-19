class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_zip_star(xs: list[A], ys: list[B]):
    for a, b in zip(*[xs, ys]):
        a.run()
        b.run()


def use_zip_star_tuple(xs: list[A], ys: list[B]):
    for a, b in zip(*(xs, ys)):
        a.run()
        b.run()


def use_zip_star_literal():
    for a, b in zip(*[[A()], [B()]]):
        a.run()
        b.run()


def use_zip_star_assigned():
    xs = [A()]
    ys = [B()]
    for a, b in zip(*[xs, ys]):
        a.run()
        b.run()


def use_zip_star_preserves_b(xs: list[B], ys: list[B]):
    for b1, b2 in zip(*[xs, ys]):
        b1.run()
        b2.run()


def use_zip_star_strict(xs: list[A], ys: list[B]):
    for a, b in zip(*[xs, ys], strict=True):
        a.run()
        b.run()
