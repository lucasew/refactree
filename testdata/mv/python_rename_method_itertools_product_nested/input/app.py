import itertools
from itertools import product


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_product_nested(xs: list[A], ys: list[A]):
    for combo in itertools.product(xs, ys):
        for a in combo:
            a.run()


def use_product_nested_imported(xs: list[A], ys: list[A]):
    for combo in product(xs, ys):
        for a in combo:
            a.run()


def use_product_nested_b(xs: list[B], ys: list[B]):
    for combo in product(xs, ys):
        for b in combo:
            b.run()


def use_product_nested_literal():
    for combo in product([A()], [A()]):
        for a in combo:
            a.run()
    for combo in itertools.product([B()], [B()]):
        for b in combo:
            b.run()


def use_product_nested_assigned():
    xs = [A()]
    ys = [A()]
    for combo in product(xs, ys):
        for a in list(combo):
            a.run()
    zs = [B()]
    ws = [B()]
    for combo in itertools.product(zs, ws):
        for b in combo:
            b.run()


def use_product_nested_next(xs: list[A], ys: list[A]):
    for combo in product(xs, ys):
        a = next(combo)
        a.run()
    for combo in itertools.product(xs, ys):
        a = next(combo)
        a.run()


def use_product_nested_preserves_b(xs: list[B], ys: list[B]):
    for combo in product(xs, ys):
        for b in combo:
            b.run()
