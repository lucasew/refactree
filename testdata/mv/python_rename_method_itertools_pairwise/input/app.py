import itertools
from itertools import pairwise


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_pairwise(items: list[A]):
    for a, nxt in itertools.pairwise(items):
        a.run()
        nxt.run()


def use_pairwise_imported(items: list[A]):
    for a, nxt in pairwise(items):
        a.run()
        nxt.run()


def use_pairwise_b(items: list[B]):
    for b, bnxt in pairwise(items):
        b.run()
        bnxt.run()


def use_pairwise_literal():
    for a, nxt in pairwise([A(), A()]):
        a.run()
        nxt.run()
    for b, bnxt in itertools.pairwise([B(), B()]):
        b.run()
        bnxt.run()


def use_pairwise_assigned():
    xs = [A(), A()]
    for a, nxt in pairwise(xs):
        a.run()
        nxt.run()
    ys = [B(), B()]
    for b, bnxt in itertools.pairwise(ys):
        b.run()
        bnxt.run()


def use_pairwise_preserves_b(items: list[B]):
    for b1, b2 in pairwise(items):
        b1.run()
        b2.run()
