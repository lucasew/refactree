class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_pop_unpack(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a, b = pairs.pop()
    a.execute()
    b.execute()


def use_pop0_unpack(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a, b = pairs.pop(0)
    a.execute()
    b.execute()


def use_pop_pair_sub(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    pair = pairs.pop()
    a = pair[0]
    a.execute()
    c = pair[1]
    c.execute()


def use_pop_pair_unpack(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    pair = pairs.pop(0)
    a, b = pair
    a.execute()
    b.execute()


def use_pop_nested(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    pair = pairs.pop()
    for a in pair:
        a.execute()
    b = next(iter(pair))
    b.execute()


def use_pop_sub_direct(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a = pairs.pop()[0]
    a.execute()
    b = pairs.pop(0)[1]
    b.execute()


def use_list_zip_pop_unpack(xs: list[A], ys: list[A]):
    a, b = list(zip(xs, ys)).pop()
    a.execute()
    b.execute()


def use_walrus_pop_pair(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    if (pair := pairs.pop()):
        a, b = pair
        a.execute()
        b.execute()
        for c in pair:
            c.execute()


def use_pop_literal():
    pairs = list(zip([A()], [A()]))
    a, b = pairs.pop()
    a.execute()
    pairs2 = list(zip([B()], [B()]))
    x, y = pairs2.pop()
    x.run()


def use_pop_unpack_b(xs: list[B], ys: list[B]):
    pairs = list(zip(xs, ys))
    x, y = pairs.pop()
    x.run()


def use_pop_pair_sub_b(xs: list[B], ys: list[B]):
    pairs = list(zip(xs, ys))
    pair = pairs.pop()
    x = pair[0]
    x.run()


def use_pop_nested_b(xs: list[B], ys: list[B]):
    pairs = list(zip(xs, ys))
    pair = pairs.pop()
    for x in pair:
        x.run()


def use_pop_preserves_b(xs: list[B], ys: list[B]):
    pairs = list(zip(xs, ys))
    pair = pairs.pop()
    x, y = pair
    x.run()
