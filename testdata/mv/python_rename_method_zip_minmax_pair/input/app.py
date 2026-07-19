class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_min_unpack(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a, b = min(pairs)
    a.run()
    b.run()


def use_max_unpack(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a, b = max(pairs)
    a.run()
    b.run()


def use_min_key_unpack(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a, b = min(pairs, key=lambda p: 0)
    a.run()
    b.run()


def use_min_pair_sub(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    pair = min(pairs)
    a = pair[0]
    a.run()
    c = pair[1]
    c.run()


def use_max_pair_unpack(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    pair = max(pairs)
    a, b = pair
    a.run()
    b.run()


def use_min_nested(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    pair = min(pairs)
    for a in pair:
        a.run()
    b = next(iter(pair))
    b.run()


def use_min_sub_direct(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a = min(pairs)[0]
    a.run()
    b = max(pairs)[1]
    b.run()


def use_list_zip_min_unpack(xs: list[A], ys: list[A]):
    a, b = min(list(zip(xs, ys)))
    a.run()
    b.run()


def use_list_zip_max_sub(xs: list[A], ys: list[A]):
    a = max(list(zip(xs, ys)))[0]
    a.run()


def use_walrus_min_pair(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    if (pair := min(pairs)):
        a, b = pair
        a.run()
        b.run()
        for c in pair:
            c.run()


def use_min_literal():
    pairs = list(zip([A()], [A()]))
    a, b = min(pairs)
    a.run()
    pairs2 = list(zip([B()], [B()]))
    x, y = min(pairs2)
    x.run()


def use_min_unpack_b(xs: list[B], ys: list[B]):
    pairs = list(zip(xs, ys))
    x, y = min(pairs)
    x.run()


def use_max_pair_sub_b(xs: list[B], ys: list[B]):
    pairs = list(zip(xs, ys))
    pair = max(pairs)
    x = pair[0]
    x.run()


def use_min_nested_b(xs: list[B], ys: list[B]):
    pairs = list(zip(xs, ys))
    pair = min(pairs)
    for x in pair:
        x.run()


def use_min_preserves_b(xs: list[B], ys: list[B]):
    pairs = list(zip(xs, ys))
    pair = min(pairs)
    x, y = pair
    x.run()
