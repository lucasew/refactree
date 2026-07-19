import itertools
from itertools import groupby


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_next_groupby_unpack(items: list[A]):
    _, ga = next(groupby(items))
    a = next(ga)
    a.run()


def use_next_itertools_groupby_unpack(items: list[A]):
    _, ga = next(itertools.groupby(items))
    a = next(ga)
    a.run()


def use_next_groupby_for(items: list[A]):
    _, ga = next(groupby(items))
    for a in ga:
        a.run()


def use_next_groupby_list(items: list[A]):
    _, ga = next(groupby(items))
    for a in list(ga):
        a.run()


def use_next_groupby_b(items: list[B]):
    _, gb = next(groupby(items))
    b = next(gb)
    b.run()


def use_next_groupby_literal():
    _, ga = next(groupby([A(), A()]))
    a = next(ga)
    a.run()
    _, gb = next(itertools.groupby([B(), B()]))
    b = next(gb)
    b.run()


def use_next_groupby_key(items: list[A]):
    _, ga = next(groupby(items, key=lambda x: 0))
    a = next(ga)
    a.run()


def use_k_ga_named(items: list[A]):
    k, ga = next(groupby(items))
    a = next(ga)
    a.run()
