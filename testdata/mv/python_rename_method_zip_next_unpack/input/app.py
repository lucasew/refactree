import itertools
from itertools import zip_longest, pairwise


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_next_zip(xs: list[A], ys: list[A]):
    a, b = next(zip(xs, ys))
    a.run()
    b.run()


def use_next_pairs(xs: list[A], ys: list[A]):
    pairs = zip(xs, ys)
    a, b = next(pairs)
    a.run()
    b.run()


def use_next_pair_local(xs: list[A], ys: list[A]):
    pairs = zip(xs, ys)
    pair = next(pairs)
    a, b = pair
    a.run()
    b.run()


def use_next_list_pattern(xs: list[A], ys: list[A]):
    [a, b] = next(zip(xs, ys))
    a.run()
    b.run()


def use_next_enumerate(xs: list[A]):
    i, a = next(enumerate(xs))
    a.run()


def use_next_zip_longest(xs: list[A], ys: list[A]):
    a, b = next(zip_longest(xs, ys))
    a.run()
    b.run()
    c, d = next(itertools.zip_longest(xs, ys))
    c.run()
    d.run()


def use_next_pairwise(xs: list[A]):
    a, b = next(pairwise(xs))
    a.run()
    b.run()
    c, d = next(itertools.pairwise(xs))
    c.run()
    d.run()


def use_next_list_zip(xs: list[A], ys: list[A]):
    pairs = list(zip(xs, ys))
    a, b = next(iter(pairs))
    a.run()
    b.run()


def use_walrus_pair_unpack(xs: list[A], ys: list[A]):
    if (pair := next(zip(xs, ys))):
        a, b = pair
        a.run()
        b.run()


def use_next_zip_b(xs: list[B], ys: list[B]):
    x, y = next(zip(xs, ys))
    x.run()


def use_next_pairs_b(xs: list[B], ys: list[B]):
    pairs = zip(xs, ys)
    x, y = next(pairs)
    x.run()


def use_next_zip_literal():
    a, b = next(zip([A()], [A()]))
    a.run()
    x, y = next(zip([B()], [B()]))
    x.run()


def use_next_zip_preserves_b(xs: list[B], ys: list[B]):
    pair = next(zip(xs, ys))
    x, y = pair
    x.run()
