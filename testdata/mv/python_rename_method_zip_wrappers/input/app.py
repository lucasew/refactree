class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_list_zip(xs: list[A], ys: list[A]):
    for a, b in list(zip(xs, ys)):
        a.run()
        b.run()


def use_tuple_zip(xs: list[A], ys: list[A]):
    for a, b in tuple(zip(xs, ys)):
        a.run()
        b.run()


def use_iter_zip(xs: list[A], ys: list[A]):
    for a, b in iter(zip(xs, ys)):
        a.run()
        b.run()


def use_reversed_zip(xs: list[A], ys: list[A]):
    for a, b in reversed(list(zip(xs, ys))):
        a.run()
        b.run()


def use_sorted_zip(xs: list[A], ys: list[A]):
    for a, b in sorted(zip(xs, ys), key=lambda p: 0):
        a.run()
        b.run()


def use_filter_zip(xs: list[A], ys: list[A]):
    for a, b in filter(None, zip(xs, ys)):
        a.run()
        b.run()


def use_list_zip_nested(xs: list[A], ys: list[A]):
    for pair in list(zip(xs, ys)):
        for a in pair:
            a.run()


def use_reversed_zip_nested(xs: list[A], ys: list[A]):
    for pair in reversed(list(zip(xs, ys))):
        for a in pair:
            a.run()


def use_list_zip_b(xs: list[B], ys: list[B]):
    for x, y in list(zip(xs, ys)):
        x.run()


def use_filter_zip_b(xs: list[B], ys: list[B]):
    for x, y in filter(None, zip(xs, ys)):
        x.run()


def use_list_zip_literal():
    for a, b in list(zip([A()], [A()])):
        a.run()
    for x, y in list(zip([B()], [B()])):
        x.run()


def use_list_zip_assigned():
    xs = [A()]
    ys = [A()]
    for a, b in list(zip(xs, ys)):
        a.run()
    zs = [B()]
    ws = [B()]
    for x, y in reversed(list(zip(zs, ws))):
        x.run()


def use_list_zip_preserves_b(xs: list[B], ys: list[B]):
    for x, y in list(zip(xs, ys)):
        x.run()
