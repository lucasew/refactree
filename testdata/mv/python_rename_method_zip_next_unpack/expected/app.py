import itertools
from itertools import zip_longest, pairwise


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_next_zip(xs: list[A], ys: list[A]):
    a, b = next(zip(xs, ys))
    a.execute()
    b.execute()


def use_next_pairs(xs: list[A], ys: list[A]):
    pairs = zip(xs, ys)
    a, b = next(pairs)
    a.execute()
    b.execute()


def use_next_pair_local(xs: list[A], ys: list[A]):
    pairs = zip(xs, ys)
    pair = next(pairs)
    a, b = pair
    a.execute()
    b.execute()


def use_next_list_pattern(xs: list[A], ys: list[A]):
    [a, b] = next(zip(xs, ys))
    a.execute()
    b.execute()


def use_next_enumerate(xs: list[A]):
    i, a = next(enumerate(xs))
    a.execute()


def use_next_zip_longest(xs: list[A], ys: list[A]):
    a, b = next(zip_longest(xs, ys))
    a.execute()
    b.execute()
    c, d = next(itertools.zip_longest(xs, ys))
    c.execute()
    d.execute()


def use_next_pairwise(xs: list[A]):
    a, b = next(pairwise(xs))
    a.execute()
    b.execute()
    c, d = next(itertools.pairwise(xs))
    c.execute()
    d.execute()


def use_next_list_zip(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a, b = next(iter(pairs))
    a.execute()
    b.execute()


def use_walrus_pair_unpack(xs: list[A], ys: list[A]):
    if (pair := next(zip(xs, ys))):
        a, b = pair
        a.execute()
        b.execute()


def use_next_zip_b(xs: list[B], ys: list[B]):
    x, y = next(zip(xs, ys))
    x.run()


def use_next_pairs_b(xs: list[B], ys: list[B]):
    pairs = zip(xs, ys)
    x, y = next(pairs)
    x.run()


def use_next_zip_literal():
    a, b = next(zip([A()], [A()]))
    a.execute()
    x, y = next(zip([B()], [B()]))
    x.run()


def use_next_zip_preserves_b(xs: list[B], ys: list[B]):
    pair = next(zip(xs, ys))
    x, y = pair
    x.run()
