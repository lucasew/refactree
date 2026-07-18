import itertools
from itertools import product, combinations, permutations, chain


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_product(xs: list[A], ys: list[A]):
    for a, b in itertools.product(xs, ys):
        a.run()
        b.run()


def use_product_imported(xs: list[A], ys: list[A]):
    for a, b in product(xs, ys):
        a.run()
        b.run()


def use_product_b(xs: list[B], ys: list[B]):
    for bx, by in product(xs, ys):
        bx.run()
        by.run()


def use_product_literal():
    for a, b in product([A()], [A()]):
        a.run()
        b.run()
    for bx, by in itertools.product([B()], [B()]):
        bx.run()
        by.run()


def use_product_assigned():
    xs = [A()]
    ys = [A()]
    for a, b in product(xs, ys):
        a.run()
        b.run()
    zs = [B()]
    ws = [B()]
    for bx, by in itertools.product(zs, ws):
        bx.run()
        by.run()


def use_combinations(items: list[A]):
    for a, b in itertools.combinations(items, 2):
        a.run()
        b.run()


def use_combinations_imported(items: list[A]):
    for a, b in combinations(items, 2):
        a.run()
        b.run()


def use_combinations_b(items: list[B]):
    for bx, by in combinations(items, 2):
        bx.run()
        by.run()


def use_permutations(items: list[A]):
    for a, b in itertools.permutations(items, 2):
        a.run()
        b.run()


def use_permutations_imported(items: list[A]):
    for a, b in permutations(items, 2):
        a.run()
        b.run()


def use_permutations_b(items: list[B]):
    for bx, by in permutations(items, 2):
        bx.run()
        by.run()


def use_from_iterable(items: list[A], more: list[A]):
    for a in itertools.chain.from_iterable([items, more]):
        a.run()


def use_from_iterable_imported(items: list[A], more: list[A]):
    for a in chain.from_iterable([items, more]):
        a.run()


def use_from_iterable_b(items: list[B], more: list[B]):
    for bx in chain.from_iterable([items, more]):
        bx.run()


def use_from_iterable_literal():
    for a in chain.from_iterable([[A()], [A()]]):
        a.run()
    for bx in itertools.chain.from_iterable([[B()], [B()]]):
        bx.run()


def use_from_iterable_assigned():
    xs = [A()]
    ys = [A()]
    for a in chain.from_iterable([xs, ys]):
        a.run()
    zs = [B()]
    ws = [B()]
    for bx in itertools.chain.from_iterable([zs, ws]):
        bx.run()
