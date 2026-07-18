import itertools
from itertools import groupby


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_groupby_key(items: list[A]):
    groupby(items, key=lambda x: x.run())


def use_groupby_key_mod(items: list[A]):
    itertools.groupby(items, key=lambda x: x.run())


def use_groupby_key_b(items: list[B]):
    groupby(items, key=lambda y: y.run())


def use_groupby_key_mod_b(items: list[B]):
    itertools.groupby(items, key=lambda y: y.run())


def use_groupby_key_for(items: list[A]):
    for k, g in groupby(items, key=lambda x: x.run()):
        for a in g:
            a.run()


def use_groupby_key_for_b(items: list[B]):
    for k, g in groupby(items, key=lambda y: y.run()):
        for b in g:
            b.run()


def use_groupby_key_assigned():
    xs = [A()]
    groupby(xs, key=lambda x: x.run())
    ys = [B()]
    groupby(ys, key=lambda y: y.run())


def use_groupby_key_literal():
    groupby([A()], key=lambda x: x.run())
    itertools.groupby([B()], key=lambda y: y.run())


def use_groupby_key_wrapper(items: list[A]):
    groupby(list(items), key=lambda x: x.run())
