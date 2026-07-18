import itertools
from itertools import tee


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_tee(items: list[A]):
    it1, it2 = itertools.tee(items)
    for a in it1:
        a.run()
    for a in it2:
        a.run()


def use_tee_imported(items: list[A]):
    it1, it2 = tee(items)
    for a in it1:
        a.run()
    for a in it2:
        a.run()


def use_tee_b(items: list[B]):
    it1, it2 = tee(items)
    for b in it1:
        b.run()


def use_tee_n(items: list[A]):
    it1, it2, it3 = tee(items, 3)
    for a in it1:
        a.run()
    for a in it2:
        a.run()
    for a in it3:
        a.run()


def use_tee_next(items: list[A]):
    it1, it2 = tee(items)
    a = next(it1)
    a.run()
    a = it2.__next__()
    a.run()


def use_tee_literal():
    it1, it2 = tee([A(), A()])
    for a in it1:
        a.run()
    it3, it4 = itertools.tee([B(), B()])
    for b in it3:
        b.run()


def use_tee_assigned():
    xs = [A()]
    it1, it2 = tee(xs)
    for a in it1:
        a.run()
    ys = [B()]
    it3, it4 = itertools.tee(ys)
    for b in it3:
        b.run()


def use_tee_preserves_b(items: list[B]):
    it1, it2 = tee(items)
    for b in it1:
        b.run()
