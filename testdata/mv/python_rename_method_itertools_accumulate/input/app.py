import itertools
from itertools import accumulate


class A:
    def run(self):
        return 1


class B:
    def run(self):
        return 2


def use_accumulate(items: list[A]):
    for a in itertools.accumulate(items):
        a.run()


def use_accumulate_imported(items: list[A]):
    for a in accumulate(items):
        a.run()


def use_accumulate_b(items: list[B]):
    for b in accumulate(items):
        b.run()


def use_accumulate_func(items: list[A]):
    for a in accumulate(items, lambda x, y: x):
        a.run()


def use_accumulate_mod_func(items: list[A]):
    for a in itertools.accumulate(items, lambda x, y: x):
        a.run()


def use_accumulate_literal():
    for a in accumulate([A()]):
        a.run()
    for b in itertools.accumulate([B()]):
        b.run()


def use_accumulate_assigned():
    xs = [A()]
    for a in accumulate(xs):
        a.run()
    ys = [B()]
    for b in itertools.accumulate(ys):
        b.run()


def use_accumulate_nested(items: list[A]):
    for a in list(accumulate(items)):
        a.run()


def use_accumulate_bind(items: list[A]):
    it = accumulate(items)
    for a in it:
        a.run()
