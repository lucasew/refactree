import itertools
from itertools import zip_longest, pairwise


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_zip_nested(xs: list[A], ys: list[A]):
    for pair in zip(xs, ys):
        for a in pair:
            a.run()


def use_zip_nested_star(xs: list[A], ys: list[A]):
    for pair in zip(*[xs, ys]):
        for a in pair:
            a.run()


def use_zip_nested_b(xs: list[B], ys: list[B]):
    for pair in zip(xs, ys):
        for b in pair:
            b.run()


def use_zip_longest_nested(xs: list[A], ys: list[A]):
    for pair in zip_longest(xs, ys):
        for a in pair:
            a.run()
    for pair in itertools.zip_longest(xs, ys):
        a = next(pair)
        a.run()


def use_zip_longest_nested_b(xs: list[B], ys: list[B]):
    for pair in zip_longest(xs, ys):
        for b in pair:
            b.run()


def use_pairwise_nested(xs: list[A]):
    for pair in pairwise(xs):
        for a in pair:
            a.run()
    for pair in itertools.pairwise(xs):
        a = next(pair)
        a.run()


def use_pairwise_nested_b(xs: list[B]):
    for pair in pairwise(xs):
        for b in pair:
            b.run()


def use_zip_nested_literal():
    for pair in zip([A()], [A()]):
        for a in pair:
            a.run()
    for pair in zip_longest([B()], [B()]):
        for b in pair:
            b.run()


def use_zip_nested_assigned():
    xs = [A()]
    ys = [A()]
    for pair in zip(xs, ys):
        for a in list(pair):
            a.run()
    zs = [B()]
    ws = [B()]
    for pair in itertools.zip_longest(zs, ws):
        for b in pair:
            b.run()


def use_zip_nested_preserves_b(xs: list[B], ys: list[B]):
    for pair in zip(xs, ys):
        for b in pair:
            b.run()
