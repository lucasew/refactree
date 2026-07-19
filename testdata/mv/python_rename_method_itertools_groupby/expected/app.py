import itertools
from itertools import groupby


class A:
    def execute(self):
        return 1


class B:
    def run(self):
        return 2


def use_groupby(items: list[A]):
    for k, g in itertools.groupby(items):
        for a in g:
            a.execute()


def use_groupby_imported(items: list[A]):
    for k, g in groupby(items):
        for a in g:
            a.execute()


def use_groupby_b(items: list[B]):
    for k, g in groupby(items):
        for b in g:
            b.run()


def use_groupby_literal():
    for k, g in groupby([A(), A()]):
        for a in g:
            a.execute()
    for k, g in itertools.groupby([B(), B()]):
        for b in g:
            b.run()


def use_groupby_assigned():
    xs = [A(), A()]
    for k, g in groupby(xs):
        for a in g:
            a.execute()
    ys = [B(), B()]
    for k, g in itertools.groupby(ys):
        for b in g:
            b.run()


def use_groupby_key(items: list[A]):
    for k, g in groupby(items, key=lambda x: 0):
        for a in g:
            a.execute()


def use_groupby_list(items: list[A]):
    for k, g in groupby(items):
        for a in list(g):
            a.execute()


def use_groupby_next(items: list[A]):
    for k, g in groupby(items):
        a = next(g)
        a.execute()


def use_groupby_preserves_b(items: list[B]):
    for k, g in groupby(items):
        for b in g:
            b.run()
