import itertools
from itertools import zip_longest


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_zip_longest(xs: list[A], ys: list[B]):
    for a, b in itertools.zip_longest(xs, ys):
        a.run()
        b.run()


def use_zip_longest_imported(xs: list[A], ys: list[B]):
    for a, b in zip_longest(xs, ys):
        a.run()
        b.run()


def use_zip_longest_fillvalue(xs: list[A], ys: list[B]):
    for a, b in zip_longest(xs, ys, fillvalue=None):
        a.run()
        b.run()


def use_zip_longest_mod_fillvalue(xs: list[A], ys: list[B]):
    for a, b in itertools.zip_longest(xs, ys, fillvalue=None):
        a.run()
        b.run()


def use_zip_longest_literal():
    for a, b in zip_longest([A()], [B()]):
        a.run()
        b.run()


def use_zip_longest_assigned():
    xs = [A()]
    ys = [B()]
    for a, b in itertools.zip_longest(xs, ys):
        a.run()
        b.run()


def use_zip_longest_preserves_b(xs: list[B], ys: list[B]):
    for b1, b2 in zip_longest(xs, ys):
        b1.run()
        b2.run()
