import itertools
from itertools import zip_longest, pairwise, product


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_zip_assign(xs: list[A], ys: list[A]):
    pairs = zip(xs, ys)
    for a, b in pairs:
        a.run()
        b.run()


def use_zip_assign_nested(xs: list[A], ys: list[A]):
    pairs = zip(xs, ys)
    for pair in pairs:
        for a in pair:
            a.run()


def use_zip_assign_b(xs: list[B], ys: list[B]):
    pairs = zip(xs, ys)
    for x, y in pairs:
        x.run()
        y.run()


def use_zip_longest_assign(xs: list[A], ys: list[A]):
    pairs = zip_longest(xs, ys)
    for a, b in pairs:
        a.run()
    pairs2 = itertools.zip_longest(xs, ys)
    for pair in pairs2:
        for a in pair:
            a.run()


def use_zip_longest_assign_b(xs: list[B], ys: list[B]):
    pairs = zip_longest(xs, ys)
    for x, y in pairs:
        x.run()


def use_pairwise_assign(xs: list[A]):
    pairs = pairwise(xs)
    for a, b in pairs:
        a.run()
        b.run()
    pairs2 = itertools.pairwise(xs)
    for pair in pairs2:
        for a in pair:
            a.run()


def use_pairwise_assign_b(xs: list[B]):
    pairs = pairwise(xs)
    for x, y in pairs:
        x.run()


def use_product_assign(xs: list[A], ys: list[A]):
    combos = product(xs, ys)
    for a, b in combos:
        a.run()
        b.run()
    combos2 = itertools.product(xs, ys)
    for combo in combos2:
        for a in combo:
            a.run()


def use_product_assign_b(xs: list[B], ys: list[B]):
    combos = product(xs, ys)
    for x, y in combos:
        x.run()


def use_zip_assign_star(xs: list[A], ys: list[A]):
    pairs = zip(*[xs, ys])
    for a, b in pairs:
        a.run()
        b.run()


def use_zip_assign_literal():
    pairs = zip([A()], [A()])
    for a, b in pairs:
        a.run()
    pairs2 = zip([B()], [B()])
    for x, y in pairs2:
        x.run()


def use_zip_assign_assigned():
    xs = [A()]
    ys = [A()]
    pairs = zip(xs, ys)
    for a, b in pairs:
        a.run()
    zs = [B()]
    ws = [B()]
    pairs2 = zip(zs, ws)
    for x, y in pairs2:
        x.run()


def use_zip_assign_preserves_b(xs: list[B], ys: list[B]):
    pairs = zip(xs, ys)
    for x, y in pairs:
        x.run()
